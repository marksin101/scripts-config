#!/usr/bin/python3

# Purpose of this script to start wireguard when the PC is connected to a foreign wifi network
# This script is meant for linux only.

# How to use: Create a service file and place it at /etc/systemd/system/ && systemctl enable <servicename>.service

# systemd service template
# [Unit]
# Description=
# Requires=network-online.target
# After=network-online.target

# [Service]
# Type=oneshot
# ExecStart= <Path to Script>


# [Install]
# WantedBy=multi-user.target

import subprocess
import re
import os
import sys
# Replace the name with the SSID (your trusted networks) that upon connection you don't want to start wiregard
ssid_to_check = ["foo", "bar"]
log_path = "/tmp/wifi_wireguard_autoconnect.log"


def write_test(s):
    file = open(log_path, "w+")
    file.write(s)
    file.write("\n")
    file.close()
    sys.stdout.write(s)


if os.geteuid() != 0:
    write_test("Please run as root. Exiting")
    os._exit(1)
try:
    cmd = "iwconfig"
    cmd_start_wg = "systemctl start wg-quick@wg0"
    proc = subprocess.Popen(cmd, shell=False, stdout=subprocess.PIPE)
    out, err = proc.communicate()
    proc.kill()
    if err is not None:
        raise RuntimeError(err)
    out = str(out).split()
    r = re.compile("ESSID*")
    ssid = list(filter(r.match, out))

    tmp = ssid[0].split("\"")
    if "ESSID:off/any" in tmp:
        write_test("Not connected to any wifi. Exiting")
        os._exit(0)
    ssid = tmp[1]
    if ssid in ssid_to_check:
        write_test("Connected to homenetwork. Nothing to do. Exiting")
        os._exit(0)
    else:
        write_test("Foreign network detected. Starting Wireguard")
        # Change the wgN accordingly
        process = subprocess.Popen(["systemctl", "start", "wg-quick@wg0"],
                                   shell=False, stdout=subprocess.PIPE)

except RuntimeError as err:
    sys.stderr.write(err)

except:
    sys.stderr.write("An error has occured")
