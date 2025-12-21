#!/bin/bash

# Install Chrome/Chromium for PropLeads SOS Scraper
# This script installs Chromium browser in WSL/Linux for the SOS business lookup feature

echo "================================================"
echo "Installing Chromium for PropLeads SOS Scraper"
echo "================================================"
echo ""

# Check if running in WSL
if grep -qi microsoft /proc/version; then
    echo "✓ Detected WSL environment"
else
    echo "✓ Detected Linux environment"
fi

echo ""
echo "This will install Chromium browser to enable business official lookup"
echo "from the NC Secretary of State website."
echo ""

# Update package lists
echo "→ Updating package lists..."
sudo apt update

# Install Chromium
echo ""
echo "→ Installing Chromium browser..."
sudo apt install -y chromium-browser

# Verify installation
if command -v chromium-browser &> /dev/null; then
    echo ""
    echo "✓ Chromium installed successfully!"
    chromium-browser --version
    echo ""
    echo "================================================"
    echo "Installation Complete!"
    echo "================================================"
    echo ""
    echo "You can now run the full scraper with SOS lookup:"
    echo "  ./scraper --county newhanover"
    echo ""
else
    echo ""
    echo "✗ Installation failed. Trying alternative method..."
    echo ""

    # Try alternative: chromium
    sudo apt install -y chromium

    if command -v chromium &> /dev/null; then
        echo ""
        echo "✓ Chromium installed successfully (as 'chromium')!"
        chromium --version
        echo ""
        # Create symlink for compatibility
        sudo ln -sf $(which chromium) /usr/bin/chromium-browser
        echo "Created symlink: chromium-browser -> chromium"
        echo ""
        echo "================================================"
        echo "Installation Complete!"
        echo "================================================"
        echo ""
        echo "You can now run the full scraper with SOS lookup:"
        echo "  ./scraper --county newhanover"
        echo ""
    else
        echo ""
        echo "✗ Installation failed. Please install manually:"
        echo "  sudo apt install chromium-browser"
        echo "or"
        echo "  sudo apt install chromium"
        exit 1
    fi
fi
