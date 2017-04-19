#!/bin/bash
#
# Docker start script.
#

DIR=$( cd "$( dirname "${BASH_SOURCE[0]}" )" && pwd )
cd $DIR

STORAGE_DISK="[[.STORAGE_DEV]]" # e.g. /dev/sdc
CONFIGFILE="./docker.conf"
VG=docker
POOL=thinpool
POOL_META=thinpoolmeta
POOL_META_ATTACH=thinpool_tmeta
PIDFILE=/var/run/docker.pid

if [ ! -f $CONFIGFILE ]; then
    echo 'config file docker.conf not exists,exit!'
    exit 2
fi

if [ "$(id -u)" != '0' ]; then
    echo "docker must be run as root"
    exit 1
fi

# link docker cli to /usr/local/bin/
ln -s $DIR/docker /usr/local/bin/docker
# link nsenter to /usr/local/bin/, for webdavd 
ln -s $DIR/nsenter /usr/local/bin/nsenter

ip link del docker0 >/dev/null 2>&1

cgroupfs_mount() {
    if grep -v '^#' /etc/fstab | grep -q cgroup \
        || [ ! -e /proc/cgroups ] \
        || [ ! -d /sys/fs/cgroup ]; then
    return
fi
if ! mountpoint -q /sys/fs/cgroup; then
    mount -t tmpfs -o uid=0,gid=0,mode=0755 cgroup /sys/fs/cgroup
fi
(
cd /sys/fs/cgroup
for sys in $(awk '!/^#/ { if ($2 != 0 && $4 == 1) print $1 }' /proc/cgroups); do
    mkdir -p $sys
    if ! mountpoint -q $sys; then
        if ! mount -n -t cgroup -o $sys cgroup $sys; then
            rmdir $sys || true
        fi
    fi
done
)

cgroup_v1_memory_path="/sys/fs/cgroup/memory/"
if [ -d "$cgroup_v1_memory_path"  ]; then
    echo 1 > /sys/fs/cgroup/memory/memory.use_hierarchy
fi

mkdir -p /sys/fs/cgroup/unified
if ! mountpoint -q /sys/fs/cgroup/unified; then
    if ! mount -t cgroup2 none /sys/fs/cgroup/unified/; then
        rmdir /sys/fs/cgroup/unified || true
    fi
fi
if mountpoint -q /sys/fs/cgroup/unified; then
    if ! echo "+memory +io" > /sys/fs/cgroup/unified/cgroup.subtree_control; then
        echo "failed to add memory/io controller for cgroup v2 for docker"
    fi
fi
mkdir -p /sys/fs/cgroup/unified/docker
}

# Check if passed in vg exists. Returns 0 if volume group exists.
vg_exists() {
    for vg_name in $(vgs --noheadings -o vg_name); do
        if [ "$vg_name" == "$VG" ]; then
            return 0
        fi
    done
    return 1
}

# Check if passed in lvm pool exists. Returns 0 if pool exists.
lvm_pool_exists() {
    local lv_data
    lv_data=$( lvs --noheadings -o lv_name,lv_attr --separator , $VG | sed -e 's/^ *//')
    if echo  $lv_data | grep -q $POOL ; then
        return 0
    fi
    return 1
}

# Check if passed in lvm pool attached or not. Returns 0 if pool exists.
lvm_pool_attach() {
    if lvs -a $VG | grep -q $POOL_META_ATTACH;then
        echo "xxxxxxx1"
        return 0
    fi
    echo "xxxxxxx2"
    return 1
}

setup_lvm_thin_pool () {

    # At this point of time, check disk mount
    if  lsblk $STORAGE_DISK -o MOUNTPOINT | grep -v 'devicemapper' -q '/';then
        echo "$STORAGE_DISK need be configured lvm thin pool, error in fstab, exit"
        exit 1
    fi

    if ! pvs --noheadings $STORAGE_DISK;then
        if ! pvcreate $STORAGE_DISK;then
            echo "could not init lvm"
            exit 1
        fi
    else
        echo "hhhhhh"
    fi

    if ! vg_exists $VG;then
        if ! vgcreate $VG $STORAGE_DISK;then
            echo "vgcreate error"
            exit 1
        fi
    else 
        echo "xxxxxxxx"
    fi

    if ! lvm_pool_exists; then
        if ! lvcreate  -n $POOL $VG -l 95%VG;then
            echo "lvcreate thin pool error"
            exit 1
        fi
        if ! lvcreate  -n $POOL_META $VG -l 1%VG;then
            echo "lvcreate thin pool meta error"
            exit 1
        fi
    fi

    if ! lvm_pool_attach;then
        if ! lvconvert -y --zero n -c 512K --thinpool $VG/$POOL --poolmetadata $VG/$POOL_META; then 
            echo "lvconvert thin pool error"
            exit 1
        fi
    fi
}

apt-get install libudev-dev -y
apt-get install libdevmapper-dev -y

setup_lvm_thin_pool
echo "setup lvm done"
cgroupfs_mount

pid=`cat $PIDFILE 2>/dev/null`

if kill -0 $pid 2>/dev/null; then
    echo "docker already running pid: ${pid}"
    exit 1
fi

args=`cat $CONFIGFILE | grep -v '^#' | tr '\n' ' '`

ulimit -n 65535

cmd="dockerd --debug $args -p $PIDFILE"
echo $cmd
PATH=$DIR:$PATH exec $cmd
