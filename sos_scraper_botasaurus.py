#!/usr/bin/env python3
"""
SOS Scraper using Botasaurus with Cloudflare bypass
"""

import json
import sys
from botasaurus.browser import browser, Driver


@browser(
    block_images=True,  # Speed up by not loading images
    headless=False,  # Must run visible to bypass Cloudflare (headless mode triggers detection)
    reuse_driver=True,  # Reuse browser instance
    output=None,  # Disable automatic output writing
    user_agent=None,  # Use botasaurus's stealth user agent
)
def scrape_sos_business(driver: Driver, business_name):
    """
    Search for a business on NC SOS website using Botasaurus
    """
    result = {
        "business_name": business_name,
        "company_officials": []
    }

    try:
        print(f"🔍 Looking up: {business_name}", file=sys.stderr)

        # Navigate with Cloudflare bypass
        print("📍 Navigating to SOS website with Cloudflare bypass...", file=sys.stderr)
        driver.get(
            "https://www.sosnc.gov/online_services/search/by_title/_Business_Registration",
            bypass_cloudflare=True
        )

        # Wait for page to load (reduced from 3s to 2s)
        driver.sleep(2)

        # Check if we bypassed Cloudflare
        page_text = driver.page_html.lower()
        if "just a moment" in page_text or ("cloudflare" in page_text and "checking" in page_text):
            print("❌ Cloudflare still blocking", file=sys.stderr)
            result["company_officials"].append({
                "title": "Result",
                "name": "Cloudflare blocked"
            })
            return result

        print("✓ Page loaded, checking for search form...", file=sys.stderr)

        # Wait for search form
        search_input = driver.wait_for_element("#SearchCriteria", wait=10)
        if not search_input:
            print("❌ Search form not found", file=sys.stderr)
            result["company_officials"].append({
                "title": "Result",
                "name": "Search form not found"
            })
            return result

        print("✓ Search form found, entering business name...", file=sys.stderr)

        # Enter business name
        search_input.type(business_name)
        driver.sleep(0.3)

        # Click submit button
        print("📝 Submitting search...", file=sys.stderr)
        if driver.is_element_present("#SubmitButton"):
            driver.click("#SubmitButton")
        else:
            print("❌ Submit button not found", file=sys.stderr)
            result["company_officials"].append({
                "title": "Result",
                "name": "Submit button not found"
            })
            return result

        # Wait for results (reduced from 3s to 2s)
        driver.sleep(2)

        # Check if any results found
        page_html = driver.page_html
        if "Records Found: 0" in page_html:
            print("✗ No records found", file=sys.stderr)
            result["company_officials"].append({
                "title": "Result",
                "name": "No match"
            })
            return result

        print("✓ Results found, clicking first result...", file=sys.stderr)

        # Try to find the first "More information" link
        first_link = driver.run_js("""
            // Find all "More information" links
            const links = Array.from(document.querySelectorAll('a.searchResultsLink'));
            const moreInfoLink = links.find(link => link.textContent.includes('More information'));
            if (moreInfoLink) {
                return moreInfoLink.href;
            }
            return null;
        """)

        if not first_link:
            print("❌ No 'More information' link found, saving HTML for debugging", file=sys.stderr)
            # Save HTML to file for debugging
            with open("debug_results.html", "w") as f:
                f.write(page_html)
            print("✓ Saved HTML to debug_results.html", file=sys.stderr)
            result["company_officials"].append({
                "title": "Result",
                "name": "No match"
            })
            return result

        print(f"✓ Found 'More information' link: {first_link}", file=sys.stderr)

        # Navigate to the business profile page
        driver.get(first_link)
        driver.sleep(2)

        # Extract officials
        print("🔍 Extracting officials...", file=sys.stderr)

        # Use JavaScript to extract officials data with addresses
        officials_data = driver.run_js("""
            const officials = [];

            // Function to extract city from address string
            function extractCity(addressText) {
                // Address format: "Street\\nCity, State Zip" or "Street\\nCity State Zip"
                const lines = addressText.split('\\n').map(l => l.trim()).filter(l => l);
                if (lines.length >= 2) {
                    // Second line should contain city, state, zip
                    const cityLine = lines[1];
                    // Extract city (everything before state abbreviation)
                    const match = cityLine.match(/^([^,]+),?\\s+[A-Z]{2}\\s+\\d{5}/);
                    if (match) {
                        return match[1].trim();
                    }
                }
                return '';
            }

            // 1. Extract from "Company officials" section (ul > li structure)
            const companyOfficialsList = document.querySelectorAll('ul > li');
            for (const li of companyOfficialsList) {
                const titleSpan = li.querySelector('span.boldSpan');
                const nameLink = li.querySelector('a[href*="/online_services/search/Business_Registration"]');
                const addressDivs = li.querySelectorAll('div.para-small');

                if (titleSpan && nameLink) {
                    const title = titleSpan.textContent.trim();
                    const name = nameLink.textContent.trim().replace(/\\s+/g, ' ');

                    // Address is in the last para-small div (after the name)
                    let city = '';
                    if (addressDivs.length >= 2) {
                        const addressText = addressDivs[addressDivs.length - 1].textContent.trim();
                        city = extractCity(addressText);
                    }

                    if (name) {
                        officials.push({ title, name, city });
                    }
                }
            }

            // 2. Extract Registered agent (top-level structure)
            const labels = document.querySelectorAll('span.boldSpan');
            for (const label of labels) {
                const labelText = label.textContent.trim().toLowerCase();

                if (labelText.includes('registered agent')) {
                    const parent = label.closest('div');
                    if (parent) {
                        const link = parent.querySelector('a[href*="/online_services/search/Business_Registration"]');
                        if (link) {
                            const name = link.textContent.trim();

                            // Find the next "mailing address" div
                            let city = '';
                            let nextDiv = parent.nextElementSibling;
                            while (nextDiv) {
                                const addressLabel = nextDiv.querySelector('span.boldSpan');
                                if (addressLabel && addressLabel.textContent.toLowerCase().includes('address')) {
                                    const addressDiv = nextDiv.querySelector('div.para-small');
                                    if (addressDiv) {
                                        const addressText = addressDiv.textContent.trim();
                                        city = extractCity(addressText);
                                        break;
                                    }
                                }
                                nextDiv = nextDiv.nextElementSibling;
                                if (!nextDiv || nextDiv.tagName === 'SECTION') break;
                            }

                            if (name) {
                                // Only add if not already in list from company officials section
                                const exists = officials.some(o => o.name === name && o.title.toLowerCase().includes('agent'));
                                if (!exists) {
                                    officials.push({
                                        title: 'Registered agent',
                                        name: name,
                                        city: city
                                    });
                                }
                            }
                        }
                    }
                }
            }

            return officials;
        """)

        if not officials_data or len(officials_data) == 0:
            print("✗ No officials found on profile page", file=sys.stderr)
            result["company_officials"].append({
                "title": "Result",
                "name": "No officials found"
            })
        else:
            result["company_officials"] = officials_data
            print(f"✓ Found {len(officials_data)} official(s)", file=sys.stderr)

    except Exception as e:
        print(f"❌ Exception: {str(e)}", file=sys.stderr)
        result["company_officials"].append({
            "title": "Error",
            "name": str(e)
        })

    return result


if __name__ == "__main__":
    if len(sys.argv) < 2:
        print("Usage: python3 sos_scraper_botasaurus.py <business_name>", file=sys.stderr)
        sys.exit(1)

    business_name = sys.argv[1]
    result = scrape_sos_business(business_name)

    # Output JSON to stdout
    print(json.dumps(result, indent=2))
