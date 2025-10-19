#!/bin/sh
BOT=tokentimeboostbot
sudo cp $BOT.service  /etc/systemd/system/
sudo chmod u+rw /etc/systemd/system/$BOT.service
sudo systemctl enable  $BOT
