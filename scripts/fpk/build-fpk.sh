#!/bin/bash
set -euo pipefail

script_dir=$(CDPATH='' cd -- "$(dirname -- "${BASH_SOURCE[0]}")" && pwd)
project_root=$(CDPATH='' cd -- "$script_dir/../.." && pwd)
template_dir="$script_dir/template"
out_dir=${FPK_OUT_DIR:-"$project_root/dist-fpk"}
image_repository=${FPK_IMAGE_REPOSITORY:-ghcr.io/nciae-zyh/stundeck}

default_version=$(sed -n 's/^[[:space:]]*"version"[[:space:]]*:[[:space:]]*"\([^"]*\)".*/\1/p' "$project_root/web/package.json" | head -n 1)
version=${FPK_VERSION:-$default_version}
image_tag=${FPK_IMAGE_TAG:-v$version}

fail() {
    printf '[fpk] 错误：%s\n' "$1" >&2
    exit 1
}

[[ "$version" =~ ^[0-9]+\.[0-9]+\.[0-9]+([+-][0-9A-Za-z.-]+)?$ ]] \
    || fail "FPK_VERSION 必须是语义版本，例如 0.1.0"
[[ "$image_repository" =~ ^[A-Za-z0-9._/-]+$ ]] \
    || fail "FPK_IMAGE_REPOSITORY 包含不支持的字符"
[[ "$image_tag" =~ ^[A-Za-z0-9._-]+$ ]] \
    || fail "FPK_IMAGE_TAG 包含不支持的字符"

find_fnpack() {
    if [ -n "${FNPACK_BIN:-}" ]; then
        local configured_fnpack=$FNPACK_BIN
        [ -x "$configured_fnpack" ] || fail "FNPACK_BIN 不存在或不可执行：$configured_fnpack"
        if [[ "$configured_fnpack" != /* ]]; then
            configured_fnpack=$(CDPATH='' cd -- "$(dirname -- "$configured_fnpack")" && pwd)/$(basename -- "$configured_fnpack")
        fi
        printf '%s\n' "$configured_fnpack"
        return
    fi
    if command -v fnpack >/dev/null 2>&1; then
        command -v fnpack
        return
    fi

    local platform arch candidate
    case "$(uname -s)" in
        Darwin) platform=darwin ;;
        Linux) platform=linux ;;
        *) platform='' ;;
    esac
    case "$(uname -m)" in
        x86_64|amd64) arch=amd64 ;;
        arm64|aarch64) arch=arm64 ;;
        *) arch='' ;;
    esac
    for candidate in \
        "$project_root"/fnpack-*"$platform"*"$arch"* \
        "$project_root"/fnpack; do
        if [ -f "$candidate" ] && [ -x "$candidate" ]; then
            printf '%s\n' "$candidate"
            return
        fi
    done
    fail "找不到 fnpack；请设置 FNPACK_BIN 或把官方二进制放到项目根目录"
}

fnpack_bin=$(find_fnpack)

if [ "${FPK_SKIP_IMAGE_CHECK:-0}" != "1" ]; then
    if command -v docker >/dev/null 2>&1 && docker buildx version >/dev/null 2>&1; then
        printf '[fpk] 验证镜像 %s:%s\n' "$image_repository" "$image_tag"
        docker buildx imagetools inspect "$image_repository:$image_tag" >/dev/null \
            || fail "镜像不存在或不可访问；请先发布镜像，或仅在离线检查时设置 FPK_SKIP_IMAGE_CHECK=1"
    else
        printf '[fpk] 警告：Docker Buildx 不可用，跳过远端镜像检查。\n' >&2
    fi
fi

mkdir -p "$out_dir"
work_dir=$(mktemp -d "$out_dir/.stundeck-fpk.XXXXXX")
cleanup() {
    rm -rf -- "$work_dir"
}
trap cleanup EXIT HUP INT TERM

cp -R "$template_dir/." "$work_dir/"

replace_placeholder() {
    local file=$1 placeholder=$2 value=$3 replacement
    replacement=${value//&/\\&}
    sed "s|{{$placeholder}}|$replacement|g" "$file" > "$file.tmp"
    mv "$file.tmp" "$file"
}

replace_placeholder "$work_dir/manifest" VERSION "$version"
replace_placeholder "$work_dir/app/docker/docker-compose.yaml" IMAGE_REPOSITORY "$image_repository"
replace_placeholder "$work_dir/app/docker/docker-compose.yaml" IMAGE_TAG "$image_tag"

chmod 0755 "$work_dir"/cmd/*
chmod 0755 "$work_dir/app/docker/entrypoint.sh"
chmod 0755 "$work_dir/app/ui/index.cgi"

printf '[fpk] 版本：%s\n' "$version"
printf '[fpk] 镜像：%s:%s\n' "$image_repository" "$image_tag"
printf '[fpk] fnpack：%s\n' "$fnpack_bin"

(
    cd "$work_dir"
    "$fnpack_bin" build
)

source_fpk="$work_dir/stundeck-fpk.fpk"
[ -f "$source_fpk" ] || fail "fnpack 完成后未找到 stundeck-fpk.fpk"
output_fpk="$out_dir/stundeck-fpk-$version.fpk"
cp "$source_fpk" "$output_fpk"

printf '[fpk] 已生成：%s\n' "$output_fpk"
if command -v sha256sum >/dev/null 2>&1; then
    sha256sum "$output_fpk"
else
    shasum -a 256 "$output_fpk"
fi
