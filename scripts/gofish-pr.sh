#!/bin/bash -ex

DIR="$( cd "$( dirname "${BASH_SOURCE[0]}" )" && pwd )"
cd ${DIR}/../

rm -rf .gofish/
mkdir -p .gofish/
cd .gofish/
trap "rm -rf ${DIR}/../.gofish/" EXIT

VERSION="$(git describe --abbrev=0)"
VERSION="${VERSION#"v"}" # remove "v" from tag

function get_checksum() {
    local platform="$1"
    local tarball="oras_${VERSION}_${platform}_amd64.tar.gz"
    curl -LO https://github.com/deislabs/oras/releases/download/v${VERSION}/${tarball}
    shasum -a 256 ${tarball} | awk '{print $1}'
    rm -rf ${tarball}
}

MAC_CHECKSUM="$(get_checksum darwin)"
LINUX_CHECKSUM="$(get_checksum linux)"
WINDOWS_CHECKSUM="$(get_checksum windows)"

rm -f ${DIR}/../oras.lua
cat << EOF > ${DIR}/../oras.lua
local name = "oras"
local org = "deislabs"
local version = "${VERSION}"

food = {
    name = name,
    description = "OCI Registry As Storage",
    license = "Apache-2.0",
    homepage = "https://github.com/" .. org .. "/" .. name,
    version = version,
    packages = {
        {
            os = "linux",
            arch = "amd64",
            url = "https://github.com/" .. org .. "/" .. name .. "/releases/download/v" .. version .. "/" .. name .. "_" .. version .. "_linux_amd64.tar.gz",
            sha256 = "${LINUX_CHECKSUM}",
            resources = {
                {
                    path = name,
                    installpath = "bin/" .. name,
                    executable = true
                }
            }
        },
        {
            os = "darwin",
            arch = "amd64",
            url = "https://github.com/" .. org .. "/" .. name .. "/releases/download/v" .. version .. "/" .. name .. "_" .. version .. "_darwin_amd64.tar.gz",
            sha256 = "${MAC_CHECKSUM}",
            resources = {
                {
                    path = name,
                    installpath = "bin/" .. name,
                    executable = true
                }
            }
        },
        {
            os = "windows",
            arch = "amd64",
            url = "https://github.com/" .. org .. "/" .. name .. "/releases/download/v" .. version .. "/" .. name .. "_" .. version .. "_windows_amd64.tar.gz",
            sha256 = "${WINDOWS_CHECKSUM}",
            resources = {
                {
                    path = name .. ".exe",
                    installpath = "bin\\\" .. name .. ".exe"
                }
            }
        }
    }
}
EOF
cat ${DIR}/../oras.lua