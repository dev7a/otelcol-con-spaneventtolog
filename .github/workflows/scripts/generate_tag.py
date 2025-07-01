#!/usr/bin/env python3
"""
Generate clean beta/rc/preview tags from branch names.

This script takes a branch name like 'beta/v0.6.0-attribute-mappings'
and generates a clean tag like 'v0.6.0-beta-attribute-mappings'
(avoiding redundant version numbers).
"""

import re
import sys
import argparse


def generate_tag(branch_name: str, base_version: str) -> tuple[str, str]:
    """
    Generate a clean tag from a beta/rc/preview branch name.
    
    Args:
        branch_name: Branch name like 'beta/v0.6.0-attribute-mappings'
        base_version: Base version like 'v0.6.0'
    
    Returns:
        Tuple of (tag, branch_type)
        Example: ('v0.6.0-beta-attribute-mappings', 'beta')
    """
    # Split branch: beta/v0.6.0-attribute-mappings
    parts = branch_name.split('/')
    if len(parts) < 2:
        raise ValueError(f"Invalid branch name format: {branch_name}")
    
    branch_type = parts[0]  # beta, rc, preview
    full_suffix = parts[1]  # v0.6.0-attribute-mappings
    
    # Remove version prefix: v0.6.0-attribute-mappings -> attribute-mappings
    feature_name = re.sub(r'^v\d+\.\d+\.\d+-', '', full_suffix)
    
    # Create clean tag: v0.6.0-beta-attribute-mappings
    tag = f'{base_version}-{branch_type}-{feature_name}'
    
    return tag, branch_type


def main():
    parser = argparse.ArgumentParser(description='Generate clean beta/rc/preview tags')
    parser.add_argument('branch_name', help='Branch name (e.g., beta/v0.6.0-attribute-mappings)')
    parser.add_argument('base_version', help='Base version (e.g., v0.6.0)')
    parser.add_argument('--format', choices=['tag', 'type', 'both'], default='both',
                       help='Output format (default: both)')
    
    args = parser.parse_args()
    
    try:
        tag, branch_type = generate_tag(args.branch_name, args.base_version)
        
        if args.format == 'tag':
            print(tag)
        elif args.format == 'type':
            print(branch_type)
        else:  # both
            print(f'{tag}|{branch_type}')
            
    except ValueError as e:
        print(f"Error: {e}", file=sys.stderr)
        sys.exit(1)


if __name__ == '__main__':
    main() 