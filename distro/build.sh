#!/bin/bash

set -x

TARGET_DIR=jukybox

rsync -a --delete pi-gen/ $TARGET_DIR/

touch $TARGET_DIR/stage3/SKIP $TARGET_DIR/stage4/SKIP $TARGET_DIR/stage5/SKIP
rm $TARGET_DIR/stage4/EXPORT* $TARGET_DIR/stage5/EXPORT* $TARGET_DIR/stage2/EXPORT_NOOBS
#rm -rf $TARGET_DIR/stage2/02-net-tweaks
rm jukybox/stage2/01-sys-tweaks/00-packages-nr
rsync -a stage2/ $TARGET_DIR/stage2/

echo 'hdmi_ignore_cec_init=1' >> $TARGET_DIR/stage1/00-boot-files/files/config.txt
echo 'hdmi_force_hotplug=1' >> $TARGET_DIR/stage1/00-boot-files/files/config.txt
echo 'hdmi_group=2' >> $TARGET_DIR/stage1/00-boot-files/files/config.txt
echo 'hdmi_mode=4' >> $TARGET_DIR/stage1/00-boot-files/files/config.txt
echo "dtoverlay=lirc-rpi,gpio_in_pin=18" >> $TARGET_DIR/stage1/00-boot-files/files/config.txt
echo "dtparam=i2c_arm=on" >> $TARGET_DIR/stage1/00-boot-files/files/config.txt

echo "IMG_NAME='Jukybox'" > $TARGET_DIR/config
echo 'APT_PROXY=http://192.168.99.100:3142' >> $TARGET_DIR/config

# Temporary
export CONTINUE=1
#docker rm pigen_work

cd cacher
docker build -t apt-cacher .
docker rm apt-cacher-run
docker run -d -p 3142:3142 --name apt-cacher-run apt-cacher
cd ..

cd $TARGET_DIR 
./build-docker.sh
