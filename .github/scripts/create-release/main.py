#!/usr/bin/env python3
# Self contained script to create a GitHub release
import os
import sys
from github import Github  # PyGithub
import re

def preflight():
    # 1. Ensure GITHUB_TOKEN is available
    token = os.environ.get("GITHUB_TOKEN")
    if not token:
        print("❌ GITHUB_TOKEN environment variable not set.", file=sys.stderr)
        sys.exit(1)
    gh = Github(token)

    # 2. Extract app version from version/version.go
    version_path = os.path.join(os.getcwd(), "version", "version.go")
    try:
        with open(version_path, "r") as f:
            content = f.read()
        match = re.search(r'Version\s*=\s*"([^"]+)"', content)
        if not match:
            raise ValueError("Version constant not found in version/version.go")
        version = match.group(1)
    except Exception as e:
        print(f"❌ Failed to extract version from version/version.go: {e}", file=sys.stderr)
        sys.exit(1)

    print(f"🏷️  Prepare Release Version: {version}")
    return version, gh

def extract_changelog(version: str) -> str:
    """
    Extracts the changelog section for the given version from CHANGELOG.md.
    Reads from '## [{version}]' until the next '## [<semver>]' line or end of file.
    Returns the extracted text (including the header line).
    """
    changelog_path = os.path.join(os.getcwd(), "CHANGELOG.md")
    if not os.path.exists(changelog_path):
        raise FileNotFoundError(f"CHANGELOG.md not found at {changelog_path}")
    with open(changelog_path, "r") as f:
        lines = f.readlines()
    # Find the start of the section
    header_pattern = re.compile(rf"^## \[{re.escape(version)}\]")
    next_header_pattern = re.compile(r"^## \[[0-9]+\.[0-9]+\.[0-9]+\]")
    start = None
    for i, line in enumerate(lines):
        if header_pattern.match(line):
            start = i
            break
    if start is None:
        raise ValueError(f"Version {version} not found in CHANGELOG.md")
    # Find the end of the section
    end = len(lines)
    for j in range(start + 1, len(lines)):
        if next_header_pattern.match(lines[j]):
            end = j
            break
    return ''.join(lines[start:end]).rstrip() + "\n"

def create_release(gh: Github, version: str, changelog: str):
    """
    Creates a GitHub release for the given version with the provided changelog.
    """
    repo_name = os.environ.get("GITHUB_REPOSITORY")
    repo = gh.get_repo(repo_name)
    # Check if a release with this tag already exists and delete it
    try:
        existing_release = repo.get_release(version)
        print(f"🗑️  Deleting existing release for tag {version}...")
        existing_release.delete_release()
    except Exception as e:
        # If not found, PyGithub throws github.GithubException.UnknownObjectException
        pass
  
    # Create the new release
    release = repo.create_git_release(
        tag=version,
        name=f"Release {version}",
        message=changelog
    )
    print(f"🚀 Created Release: {release.html_url}")
    return release


if __name__ == "__main__":
    version, gh = preflight()
    changelog_section = extract_changelog(version)
    print(changelog_section)
    release = create_release(gh, version, changelog_section)

    # Set GitHub Actions env vars
    github_env = os.environ.get('GITHUB_ENV')
    if github_env:
        os.system(f'echo "RELEASE_VERSION={version}" >> "$GITHUB_ENV"')
        os.system(f'echo "RELEASE_LINK={release.html_url}" >> "$GITHUB_ENV"')
        print(f"Set RELEASE_VERSION={version} in $GITHUB_ENV (via shell)")
        print(f"Set RELEASE_LINK={release.html_url} in $GITHUB_ENV (via shell)")
    else:
        print("GITHUB_ENV not set, skipping env export.")