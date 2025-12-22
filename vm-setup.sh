#!/bin/bash
# Setup script for Debian 13 VM
# Run this inside the VM after first boot

set -e

echo "Mockelot VM Setup Script"
echo "========================"
echo ""

# Mount shared directory
echo "Step 1: Mounting shared directory..."
sudo mkdir -p /mnt/mockelot
if ! mountpoint -q /mnt/mockelot; then
    sudo mount -t 9p -o trans=virtio,version=9p2000.L mockelot_src /mnt/mockelot
    echo "✓ Mounted /mnt/mockelot"
else
    echo "✓ /mnt/mockelot already mounted"
fi

# Verify mount
ls -la /mnt/mockelot > /dev/null 2>&1
if [ $? -eq 0 ]; then
    echo "✓ Shared directory accessible"
else
    echo "✗ Error: Cannot access shared directory"
    exit 1
fi

# Make mount permanent
if ! grep -q "mockelot_src" /etc/fstab; then
    echo "Adding mount to /etc/fstab..."
    echo "mockelot_src /mnt/mockelot 9p trans=virtio,version=9p2000.L,_netdev 0 0" | sudo tee -a /etc/fstab
    echo "✓ Mount will persist across reboots"
fi

# Update system
echo ""
echo "Step 2: Updating system..."
sudo apt update
sudo apt upgrade -y

# Install build dependencies using the script from shared directory
echo ""
echo "Step 3: Installing build dependencies..."
cd /mnt/mockelot
sudo ./install-deps.sh

# Install Go
echo ""
echo "Step 4: Installing Go..."
if ! command -v go &> /dev/null; then
    cd ~
    wget -q https://go.dev/dl/go1.21.5.linux-amd64.tar.gz
    sudo tar -C /usr/local -xzf go1.21.5.linux-amd64.tar.gz
    rm go1.21.5.linux-amd64.tar.gz

    # Add to PATH
    if ! grep -q "/usr/local/go/bin" ~/.bashrc; then
        echo 'export PATH=/usr/local/go/bin:$HOME/go/bin:$PATH' >> ~/.bashrc
    fi
    export PATH=/usr/local/go/bin:$HOME/go/bin:$PATH

    echo "✓ Go installed: $(go version)"
else
    echo "✓ Go already installed: $(go version)"
fi

# Install Wails
echo ""
echo "Step 5: Installing Wails..."
if ! command -v ~/go/bin/wails &> /dev/null; then
    export PATH=/usr/local/go/bin:$HOME/go/bin:$PATH
    go install github.com/wailsapp/wails/v2/cmd/wails@latest
    echo "✓ Wails installed"
else
    echo "✓ Wails already installed"
fi

# Install Node.js
echo ""
echo "Step 6: Installing Node.js..."
if ! command -v node &> /dev/null; then
    curl -fsSL https://deb.nodesource.com/setup_18.x | sudo bash -
    sudo apt-get install -y nodejs
    echo "✓ Node.js installed: $(node --version)"
else
    echo "✓ Node.js already installed: $(node --version)"
fi

# Verify everything
echo ""
echo "Step 7: Verifying installation..."
cd /mnt/mockelot

# Check platform
./detect-platform.sh --verbose

# Test build (optional - commented out by default)
# echo ""
# echo "Step 8: Testing build..."
# make clean
# make linux

echo ""
echo "✓✓✓ VM Setup Complete! ✓✓✓"
echo ""
echo "Your mockelot directory is mounted at: /mnt/mockelot"
echo ""
echo "To build:"
echo "  cd /mnt/mockelot"
echo "  make linux              # Native build"
echo "  make appimage           # AppImage build"
echo ""
echo "To test the crash:"
echo "  cd /mnt/mockelot"
echo "  make clean && make linux"
echo "  ./build/bin/mockelot"
echo ""
echo "Note: Any changes made in /mnt/mockelot are reflected on your host machine!"
