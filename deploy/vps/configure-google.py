#!/usr/bin/env python3
"""Enter Google settings privately on the VPS; never print the app password."""
import getpass
import os
from pathlib import Path
import re
import sys

env_path = (Path(__file__).resolve().parent / '.env').resolve()
if not env_path.exists():
    sys.exit('Run init-env.sh first.')
if not sys.stdin.isatty():
    sys.exit('Use an interactive terminal so the Google app password stays hidden.')

email_pattern = re.compile(r'[^\s@]+@[^\s@]+\.[^\s@]+')
sender = input('Gmail/Workspace sender address: ').strip()
admin = input('First administrator email (Enter = sender): ').strip() or sender
if not email_pattern.fullmatch(sender) or not email_pattern.fullmatch(admin):
    sys.exit('Enter valid mailbox addresses.')
password = getpass.getpass('Google app password (hidden; not your normal password): ').replace(' ', '')
if len(password) != 16 or not password.isascii() or not password.isalnum():
    sys.exit('Expected a 16-character Google app password; settings were not saved.')
client_id = input('Google Web OAuth client ID (Enter = configure later): ').strip()
if client_id and (not client_id.endswith('.apps.googleusercontent.com') or any(ch.isspace() for ch in client_id)):
    sys.exit('Expected a Google Web OAuth client ID; settings were not saved.')

updates = {
    'ACME_EMAIL': admin,
    'KG_SMTP_HOST': 'smtp.gmail.com',
    'KG_SMTP_PORT': '587',
    'KG_SMTP_USER': sender,
    'KG_SMTP_PASSWORD': password,
    'KG_SMTP_FROM': sender,
    'KG_BOOTSTRAP_ADMIN_EMAIL': admin,
}
if client_id:
    updates['KG_GOOGLE_CLIENT_ID'] = client_id
lines = env_path.read_text().splitlines()
remaining = dict(updates)
for index, line in enumerate(lines):
    key, separator, _ = line.partition('=')
    if separator and key in remaining:
        # These validated mailbox, app-password and client-ID values need no
        # dotenv interpolation; quote literally to avoid special characters.
        value = remaining.pop(key).replace("'", "\\'")
        lines[index] = f"{key}='{value}'"
for key, value in remaining.items():
    lines.append(f"{key}='{value}'")
temporary = env_path.with_suffix('.new')
descriptor = os.open(temporary, os.O_WRONLY | os.O_CREAT | os.O_EXCL, 0o600)
try:
    with os.fdopen(descriptor, 'w') as output:
        output.write('\n'.join(lines) + '\n')
    temporary.replace(env_path)
finally:
    temporary.unlink(missing_ok=True)
print('Saved Google settings in the private VPS environment; no credentials were printed.')
