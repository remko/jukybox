#!/bin/bash -e

install -m 755 files/jukybox ${ROOTFS_DIR}/home/pi/
install -m 755 files/jukybox.sh ${ROOTFS_DIR}/home/pi/
install -m 644 files/iMac.lircd.conf ${ROOTFS_DIR}/etc/lirc/lircd.conf.d/
echo "lirc_dev" >> ${ROOTFS_DIR}/etc/modules
echo "lirc_rpi gpio_in_pin=18" >> ${ROOTFS_DIR}/etc/modules
echo 'if [[ `tty` == "/dev/tty1" ]]; then ./jukybox.sh; fi' >> ${ROOTFS_DIR}/home/pi/.bashrc
sed ${ROOTFS_DIR}/etc/lirc/lirc_options.conf -i -e "s#^driver.*#driver = default#"
sed ${ROOTFS_DIR}/etc/lirc/lirc_options.conf -i -e "s#^device.*#device = /dev/lirc0#"
sed ${ROOTFS_DIR}/etc/fstab -i -e "s#vfat.*defaults#vfat ro,defaults#"
sed ${ROOTFS_DIR}/etc/fstab -i -e "s#ext4.*defaults#ext4 ro,defaults#"

ln -fs /etc/systemd/system/autologin@.service ${ROOTFS_DIR}/etc/systemd/system/getty.target.wants/getty@tty1.service

mkdir -p ${ROOTFS_DIR}/media

sed ${ROOTFS_DIR}/lib/systemd/system/systemd-udevd.service -i -e "s#^MountFlags=.*#MountFlags=shared#"

on_chroot << EOF
systemctl disable dhcpcd
systemctl disable regenerate_ssh_host_keys
EOF
