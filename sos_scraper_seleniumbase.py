#!/usr/bin/env python3
"""
SOS Scraper using SeleniumBase UC Mode with xvfb for headless-like operation
Alternative to botasaurus scraper - uses UC Mode to bypass Cloudflare
"""

import json
import sys
from seleniumbase import SB


def scrape_sos_business(sb, business_name):
    """
    Search for a business on NC SOS website using SeleniumBase UC Mode
    """
    result = {
        "business_name": business_name,
        "company_officials": []
    }

    try:
        print(f"🔍 Looking up: {business_name}", file=sys.stderr)

        # Navigate with UC Mode reconnect to bypass Cloudflare
        print("📍 Navigating to SOS website with UC Mode...", file=sys.stderr)
        url = "https://www.sosnc.gov/online_services/search/by_title/_Business_Registration"
        sb.uc_open_with_reconnect(url, reconnect_time=4)

        # Handle any Cloudflare CAPTCHA if it appears
        print("🔐 Checking for Cloudflare protection...", file=sys.stderr)
        sb.sleep(2)

        # Try to handle CAPTCHA if present (this is smart enough to skip if not needed)
        try:
            sb.uc_gui_click_captcha()
        except Exception as e:
            print(f"ℹ️  No CAPTCHA detected or already bypassed: {e}", file=sys.stderr)

        # Check if we bypassed Cloudflare
        page_text = sb.get_page_source().lower()
        if "just a moment" in page_text or ("cloudflare" in page_text and "checking" in page_text):
            print("❌ Cloudflare still blocking", file=sys.stderr)
            result["company_officials"].append({
                "title": "Result",
                "name": "Cloudflare blocked"
            })
            return result

        print("✓ Page loaded, checking for search form...", file=sys.stderr)

        # Wait for search form
        try:
            sb.wait_for_element("#SearchCriteria", timeout=10)
        except Exception:
            print("❌ Search form not found", file=sys.stderr)
            result["company_officials"].append({
                "title": "Result",
                "name": "Search form not found"
            })
            return result

        print("✓ Search form found, entering business name...", file=sys.stderr)

        # Enter business name
        sb.type("#SearchCriteria", business_name)
        sb.sleep(0.3)

        # Click submit button using UC click for stealth
        print("📝 Submitting search...", file=sys.stderr)
        try:
            sb.wait_for_element("#SubmitButton", timeout=5)
            sb.uc_click("#SubmitButton", reconnect_time=2)
        except Exception:
            print("❌ Submit button not found", file=sys.stderr)
            result["company_officials"].append({
                "title": "Result",
                "name": "Submit button not found"
            })
            return result

        # Wait for results
        sb.sleep(2)

        # Check if any results found
        page_html = sb.get_page_source()
        if "Records Found: 0" in page_html:
            print("✗ No records found", file=sys.stderr)
            result["company_officials"].append({
                "title": "Result",
                "name": "No match"
            })
            return result

        print("✓ Results found, clicking first result...", file=sys.stderr)

        # Try to find the first "More information" link
        first_link = sb.execute_script("""
            // Find all "More information" links
            const links = Array.from(document.querySelectorAll('a.searchResultsLink'));
            const moreInfoLink = links.find(link => link.textContent.includes('More information'));
            if (moreInfoLink) {
                return moreInfoLink.href;
            }
            return null;
        """)

        if not first_link:
            print("❌ No 'More information' link found", file=sys.stderr)
            result["company_officials"].append({
                "title": "Result",
                "name": "No match"
            })
            return result

        print(f"✓ Found 'More information' link: {first_link}", file=sys.stderr)

        # Navigate to the business profile page using UC mode
        sb.uc_open_with_reconnect(first_link, reconnect_time=2)
        sb.sleep(2)

        # Extract officials
        print("🔍 Extracting officials...", file=sys.stderr)

        # Use JavaScript to extract officials data with addresses
        officials_data = sb.execute_script("""
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
        import traceback
        traceback.print_exc(file=sys.stderr)
        result["company_officials"].append({
            "title": "Error",
            "name": str(e)
        })

    return result


if __name__ == "__main__":
    if len(sys.argv) < 2:
        print("Usage: python3 sos_scraper_seleniumbase.py <business_name>", file=sys.stderr)
        sys.exit(1)

    business_name = sys.argv[1]

    # Use SeleniumBase UC Mode with xvfb for headless-like operation
    # uc=True: Enables undetected mode to bypass Cloudflare
    # xvfb=True: Uses virtual display on Linux (no visible window once xvfb is installed)
    # incognito=True: Increases stealth
    # test=False: Disable test runner output for clean JSON parsing
    with SB(uc=True, xvfb=True, incognito=True, test=False) as sb:
        result = scrape_sos_business(sb, business_name)

        # Output JSON to stdout
        print(json.dumps(result, indent=2))
