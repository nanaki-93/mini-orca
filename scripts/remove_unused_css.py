#!/usr/bin/env python3
"""Remove unused CSS classes from main.css"""

import re
import sys

# Read unused classes
with open('/tmp/truly_unused_final.txt', 'r') as f:
    unused_classes = set(line.strip() for line in f if line.strip())

# Read CSS file
with open('internal/api/static/css/main.css', 'r') as f:
    css_content = f.read()

# Find all CSS rules and their selectors
# Pattern to match CSS rules: .classname { ... }
css_pattern = re.compile(r'\.([a-zA-Z][a-zA-Z0-9_-]*)\s*\{[^}]*\}', re.DOTALL)

def remove_unused_classes(match):
    """Remove CSS rules for unused classes"""
    selector = match.group(0)
    class_name = match.group(1)
    
    if class_name in unused_classes:
        return ''
    return selector

# Remove unused CSS rules
new_css = css_pattern.sub(remove_unused_classes, css_content)

# Clean up extra blank lines
new_css = re.sub(r'\n{3,}', '\n\n', new_css)

# Write back
with open('internal/api/static/css/main.css', 'w') as f:
    f.write(new_css)

print(f"Removed {len(unused_classes)} unused CSS classes")
