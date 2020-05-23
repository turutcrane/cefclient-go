#!/bin/bash

CEFGIT=/c/DiskC/dev/source/cef_git/
# windres.exe --output-format=coff -o cefclient.syso -I $CEFGIT -I $CEF_BINARY -I $CEFGIT/tests/cefclient/resources/win  cefclient.rc
windres.exe --output-format=coff -o cefclient.syso -I $CEF_BINARY -I $CEFGIT/tests/cefclient/resources/win  cefclient.rc
