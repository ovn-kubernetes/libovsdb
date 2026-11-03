#!/usr/bin/env python3
import requests
import json
import sys
import re
import argparse
from packaging.version import Version, InvalidVersion
from collections import defaultdict

OVS_TAGS_URL = "https://api.github.com/repos/openvswitch/ovs/tags?per_page=100"
VERSIONS_PATH = "ovs/versions.json"
MAKEFILE_PATH = "Makefile"


def parse_ovs_version(tag):
    m = re.match(r"^v(\d+)\.(\d+)\.(\d+)$", tag)
    if not m:
        return None
    try:
        return Version(f"{m.group(1)}.{m.group(2)}.{m.group(3)}")
    except InvalidVersion:
        return None


def fetch_ovs_tags():
    tags = []
    url = OVS_TAGS_URL
    while url:
        resp = requests.get(url)
        resp.raise_for_status()
        data = resp.json()
        tags.extend([t['name'] for t in data])
        if 'next' in resp.links:
            url = resp.links['next']['url']
        else:
            url = None
    return tags


def get_latest_patches(tags):
    versions = [parse_ovs_version(tag) for tag in tags]
    versions = [v for v in versions if v is not None]
    groups = defaultdict(list)
    for v in versions:
        groups[(v.major, v.minor)].append(v)
    latest = []
    for key in groups:
        latest.append(max(groups[key]))
    latest.sort(reverse=True)
    seen = set()
    result = []
    for v in latest:
        key = (v.major, v.minor)
        if key not in seen:
            seen.add(key)
            result.append(v)
        if len(result) == 3:
            break
    return result


def get_latest_version(tags):
    versions = [parse_ovs_version(tag) for tag in tags]
    versions = [v for v in versions if v is not None]
    if not versions:
        return None
    return max(versions)


def read_versions():
    with open(VERSIONS_PATH) as f:
        return json.load(f)


def write_versions(images):
    with open(VERSIONS_PATH, 'w') as f:
        json.dump(images, f, indent=2)
        f.write('\n')


def update_versions(images, latest_versions):
    # Keep the rolling master image and any metadata for unchanged releases.
    new_images = [image for image in images if image['ovs_version'] == 'master']
    for v in latest_versions:
        tag = f"v{v}"
        found = next(
            (img for img in images if img['ovs_version'] == tag), None)
        new_images.append(found if found is not None else
                          {'ovs_version': tag, 'tag': str(v)})
    return new_images


def read_makefile():
    with open(MAKEFILE_PATH) as f:
        lines = f.readlines()
    return lines


def write_makefile(lines):
    with open(MAKEFILE_PATH, 'w') as f:
        f.writelines(lines)


def update_makefile_ovs_version(lines, latest_version):
    updated = False
    pattern = re.compile(r'^(OVS_VERSION\s*\?=\s*)v\d+\.\d+\.\d+\s*$')
    replacement = f"OVS_VERSION ?= v{latest_version}\n"
    new_lines = lines[:]
    if lines and pattern.match(lines[0]):
        if lines[0] != replacement:
            new_lines[0] = replacement
            updated = True
    return new_lines, updated


def main():
    parser = argparse.ArgumentParser(
        description="Check and update OVS versions in ovs/versions.json and Makefile")
    parser.add_argument('--dry-run', action='store_true',
                        help='Show the would-be new ovs/versions.json and Makefile to stdout if a change would be made')
    args = parser.parse_args()

    tags = fetch_ovs_tags()
    latest_ovs = get_latest_patches(tags)
    latest_version = get_latest_version(tags)

    images = read_versions()
    new_images = update_versions(images, latest_ovs)
    versions_updated = images != new_images

    makefile_lines = read_makefile()
    new_makefile_lines, makefile_updated = update_makefile_ovs_version(
        makefile_lines, latest_version)

    if versions_updated or makefile_updated:
        if args.dry_run:
            if versions_updated:
                print(f"# Would update {VERSIONS_PATH} to:")
                print(json.dumps(new_images, indent=2))
            if makefile_updated:
                print("# Would update Makefile to:")
                print(new_makefile_lines[0].rstrip())
            sys.exit(2)
        else:
            if versions_updated:
                write_versions(new_images)
                print(f"{VERSIONS_PATH} updated with the latest 3 OVS releases.")
            if makefile_updated:
                write_makefile(new_makefile_lines)
                print("Makefile OVS_VERSION updated to latest OVS release.")
            sys.exit(2)
    else:
        print(f"{VERSIONS_PATH} and Makefile are up to date with the latest OVS releases.")
        sys.exit(0)


if __name__ == "__main__":
    main()
