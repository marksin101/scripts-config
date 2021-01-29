#!/bin/python3

# The Purpose of this script is to periodically run smart health check and send email that is gpg encrypted to alert users if disks have failed
# Prerequisites: Linux only, required to install smartmontools, python-gnupg(installed globally via sudo -H pip3 install python-gnupg), run this script as root
# How to use: place it in /etc/cron.daily/  &&  chmod +x <script name>
import smtplib
import ssl
import gnupg
import subprocess
import os
# For this to work, you need to turn google less secure access
sender_email = ""
receiver_email = ""
# password of the sender email
password = ""
gpghome = "/home/XXXX/.gnupg"
disks_to_chk = ["/dev/foo", "/dev/bar"]


def sendmail(title: str, content: str):
    gpg = gnupg.GPG(gnupghome=gpghome)
    port = 587  # For starttls
    smtp_server = "smtp.gmail.com"
    encrypted_message = gpg.encrypt(content, receiver_email)
    message = 'Subject: {}\n\n{}'.format(title, str(encrypted_message))
    context = ssl.create_default_context()
    with smtplib.SMTP(smtp_server, port) as server:
        server.ehlo()
        server.starttls(context=context)
        server.ehlo()
        server.login(sender_email, password)
        server.sendmail(sender_email, receiver_email, message)
        server.close()


def health_chk(disk: str) -> bool:
    proc = subprocess.Popen(["smartctl", "-H", disk], stdout=subprocess.PIPE)
    std, _ = proc.communicate()
    tmp = str(std).split()
    status = tmp[len(tmp)-1].split("\\")[0]
    if status == "PASSED":
        return True
    else:
        return False


def get_details(disk: str) -> str:
    message = ""
    proc = subprocess.Popen(
        ["smartctl", "--all", disk], stdout=subprocess.PIPE)
    while True:
        line = proc.stdout.readline()
        if not line:
            break
        message += line.rstrip().decode("utf-8") + "\n"

    return message


if __name__ == "__main__":
    if os.geteuid() != 0:
        os._exit(1)
    failed_disks = []
    message = ""
    for disk in disks_to_chk:
        if not health_chk(disk):
            failed_disks.append(disk)
    if failed_disks:
        for disk in failed_disks:
            message += "The following disks have failed: %s" % disk
        message += "\n##########################################################################################################\n"
        for disk in failed_disks:
            message += get_details(disk)
            message += "\n##########################################################################################################\n"
        sendmail("Emergent: Disks have Failed SmartTest", message)
