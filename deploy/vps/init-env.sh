#!/usr/bin/env bash
set -euo pipefail
kg_vps_dir="$(cd -- "$(dirname -- "${BASH_SOURCE[0]}")" && pwd)"
kg_vps_env="$kg_vps_dir/.env"
if [[ -e "$kg_vps_env" ]]; then
  echo 'deploy/vps/.env already exists; preserving it.'
  exit 0
fi
command -v openssl >/dev/null
umask 077
cp -- "$kg_vps_dir/.env.example" "$kg_vps_env"
while IFS= read -r kg_vps_key; do
  case "$kg_vps_key" in
    KG_RESEND_WEBHOOK_SECRET)
      kg_vps_value="whsec_$(openssl rand -base64 32)" ;;
    KG_TOTP_ENCRYPTION_KEY|KG_DAILY_WEBHOOK_SECRET)
      kg_vps_value="$(openssl rand -base64 32)" ;;
    *) kg_vps_value="$(openssl rand -hex 32)" ;;
  esac
  sed -i "s|^${kg_vps_key}=$|${kg_vps_key}=${kg_vps_value}|" "$kg_vps_env"
done <<'KEYS'
POSTGRES_PASSWORD
KG_TOTP_ENCRYPTION_KEY
KG_S3_ACCESS_KEY_ID
KG_S3_SECRET_ACCESS_KEY
KG_RAZORPAY_WEBHOOK_SECRET
KG_RESEND_WEBHOOK_SECRET
KG_DAILY_WEBHOOK_SECRET
KEYS
echo 'Created private deploy/vps/.env with new credentials.'
echo 'Fill in domains, ACME_EMAIL and SMTP settings before starting production.'
