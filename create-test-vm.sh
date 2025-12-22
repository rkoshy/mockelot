#!/bin/bash
# Create Debian 13 VM for Mockelot testing

set -e

# Use system connection (not session)
export LIBVIRT_DEFAULT_URI=qemu:///system

ISO_PATH="$HOME/Downloads/iso/debian-13-trixie-netinst.iso"
VM_NAME="mockelot-debian13-test"
DISK_SIZE=20  # GB
RAM=8192      # MB
CPUS=4

# Host directory to share with VM
SHARED_DIR="/ubuntu-drive/home/renny/repositories/tools/mockelot"
MOUNT_TAG="mockelot_src"

# Check if ISO exists
if [ ! -f "$ISO_PATH" ]; then
    echo "Error: ISO not found at $ISO_PATH"
    echo "Please ensure the ISO download completed successfully"
    exit 1
fi

# Use br-int-network (already active)
NETWORK="br-int-network"

# Check if VM already exists
if virsh list --all | grep -q "$VM_NAME"; then
    echo "VM $VM_NAME already exists"
    read -p "Delete and recreate? [y/N] " -n 1 -r
    echo
    if [[ $REPLY =~ ^[Yy]$ ]]; then
        virsh destroy "$VM_NAME" 2>/dev/null || true
        virsh undefine "$VM_NAME" --remove-all-storage 2>/dev/null || true
    else
        echo "Aborted"
        exit 1
    fi
fi

echo "Creating VM: $VM_NAME"
echo "  CPUs: $CPUS"
echo "  RAM: ${RAM}MB"
echo "  Disk: ${DISK_SIZE}GB"
echo "  ISO: $ISO_PATH"
echo "  Shared Directory: $SHARED_DIR"
echo "  Mount Tag: $MOUNT_TAG"
echo ""

# Create VM with virt-install
virt-install \
    --name "$VM_NAME" \
    --ram $RAM \
    --vcpus $CPUS \
    --disk size=$DISK_SIZE,format=qcow2 \
    --os-variant debian11 \
    --cdrom "$ISO_PATH" \
    --network network=$NETWORK \
    --filesystem type=mount,mode=passthrough,source=$SHARED_DIR,target=$MOUNT_TAG \
    --graphics vnc,listen=127.0.0.1 \
    --console pty,target_type=serial \
    --noautoconsole

echo ""
echo "✓ VM created successfully!"
echo ""
echo "To connect to the VM:"
echo "  virt-viewer $VM_NAME"
echo ""
echo "Or connect via VNC:"
VNC_PORT=$(virsh vncdisplay "$VM_NAME" 2>/dev/null | cut -d: -f2)
if [ -n "$VNC_PORT" ]; then
    echo "  VNC localhost:$((5900 + VNC_PORT))"
fi
echo ""
echo "To manage the VM:"
echo "  virsh start $VM_NAME      # Start VM"
echo "  virsh shutdown $VM_NAME   # Shutdown VM"
echo "  virsh destroy $VM_NAME    # Force stop VM"
echo "  virsh console $VM_NAME    # Serial console"
echo ""
echo "Installation will start automatically."
echo "Suggested install options:"
echo "  - Hostname: mockelot-test"
echo "  - User: tester / password: test"
echo "  - Partitioning: Use entire disk (simple)"
echo "  - Software: Standard system utilities + SSH server"
echo ""
echo "After installation, mount the shared directory:"
echo "  sudo mkdir -p /mnt/mockelot"
echo "  sudo mount -t 9p -o trans=virtio,version=9p2000.L $MOUNT_TAG /mnt/mockelot"
echo ""
echo "To make it permanent, add to /etc/fstab:"
echo "  $MOUNT_TAG /mnt/mockelot 9p trans=virtio,version=9p2000.L,_netdev 0 0"
