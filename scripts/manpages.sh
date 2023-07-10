#!/bin/sh
# scripts/manpages.sh
set -e
rm -rf manpages
mkdir manpages
for sh in bash zsh; do
	go run main.go man manpages
done
