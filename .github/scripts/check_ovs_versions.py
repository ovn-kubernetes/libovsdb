#!/usr/bin/env python3
import requests
import yaml
import sys
import re
import argparse
from packaging.version import Version, InvalidVersion
from collections import defaultdict

OVS_TAGS_URL = "https://api.github.com/repos/openvswitch/ovs/tags?per_page=100"
IMAGES_YML_PATH = ".github/workflows/images.yml"
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


def read_images_yml():
    with open(IMAGES_YML_PATH) as f:
        yml = yaml.safe_load(f)
    return yml


def write_images_yml(yml):
    with open(IMAGES_YML_PATH, 'w') as f:
        yaml.dump(yml, f, sort_keys=False)


def update_images_yml_versions(yml, latest_versions):
    updated = False
    images = yml['jobs']['build']['strategy']['matrix']['image']
    new_images = []
    for image in images:
        if image['ovs_version'] == 'master':
            new_images.append(image)
    for v in latest_versions:
        tag = f"v{v}"
        tag_str = str(v)
        found = next(
            (img for img in images if img['ovs_version'] == tag), None)
        if found:
            new_images.append(found)
        else:
            new_images.append({'ovs_version': tag, 'tag': tag_str})
    old_set = set((img['ovs_version'], img.get('tag', ''))
                  for img in images if img['ovs_version'] != 'master')
    new_set = set((img['ovs_version'], img.get('tag', ''))
                  for img in new_images if img['ovs_version'] != 'master')
    if old_set != new_set:
        yml['jobs']['build']['strategy']['matrix']['image'] = new_images
        updated = True
    return updated


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
        description="Check and update OVS versions in images.yml and Makefile")
    parser.add_argument('--dry-run', action='store_true',
                        help='Show the would-be new images.yml and Makefile to stdout if a change would be made')
    args = parser.parse_args()

    tags = fetch_ovs_tags()
    latest_ovs = get_latest_patches(tags)
    latest_version = get_latest_version(tags)

    yml = read_images_yml()
    yml_updated = update_images_yml_versions(yml, latest_ovs)

    makefile_lines = read_makefile()
    new_makefile_lines, makefile_updated = update_makefile_ovs_version(
        makefile_lines, latest_version)

    if yml_updated or makefile_updated:
        if args.dry_run:
            if yml_updated:
                print("# Would update .github/workflows/images.yml to:")
                print(yaml.dump(yml, sort_keys=False))
            if makefile_updated:
                print("# Would update Makefile to:")
                print(new_makefile_lines[0].rstrip())
            sys.exit(2)
        else:
            if yml_updated:
                write_images_yml(yml)
                print("images.yml updated with the latest 3 OVS releases.")
            if makefile_updated:
                write_makefile(new_makefile_lines)
                print("Makefile OVS_VERSION updated to latest OVS release.")
            sys.exit(2)
    else:
        print("images.yml and Makefile are up to date with the latest OVS releases.")
        sys.exit(0)


if __name__ == "__main__":
    main()
