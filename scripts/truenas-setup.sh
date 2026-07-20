#!/usr/bin/env bash
# TitipDong — TrueNAS one-time setup script.
#
# Creates a secrets file on the TrueNAS host that the titipdong container reads
# via `env_file:` in docker-compose. This way you can edit the app YAML via the
# TrueNAS UI later WITHOUT losing your secrets (the YAML never contains them).
#
# Run this on the TrueNAS host via SSH. It is safe to re-run.
#
# Usage:
#   sudo bash truenas-setup.sh
#
# After running, deploy the app via TrueNAS "Install via YAML" using
# docker-compose.truenas.yml.

set -euo pipefail

ENV_PATH="/mnt/.ix-applications/titipdong.env"
DB_NAME="titipdong"

GREEN='\033[0;32m'
YELLOW='\033[0;33m'
RED='\033[0;31m'
NC='\033[0m'

echo -e "${YELLOW}=== TitipDong TrueNAS setup ===${NC}"
echo ""

if [ "$(id -u)" -ne 0 ]; then
    echo -e "${RED}Please run as root (use sudo).${NC}" >&2
    exit 1
fi

if [ ! -d /mnt/.ix-applications ]; then
    echo -e "${RED}/mnt/.ix-applications not found. Are you on TrueNAS SCALE?${NC}" >&2
    exit 1
fi

if [ -f "$ENV_PATH" ]; then
    echo -e "${YELLOW}[$ENV_PATH already exists]${NC}"
    echo "Existing secrets will NOT be overwritten."
    echo "To regenerate, delete the file first: sudo rm $ENV_PATH"
    echo ""
    echo "Current values (names only, values hidden):"
    grep -oE '^[A-Z_]+=' "$ENV_PATH" | sed 's/^/  - /'
    echo ""
    echo -e "${GREEN}Secrets file OK. You can now deploy the app via 'Install via YAML'.${NC}"
    exit 0
fi

echo "Creating $ENV_PATH ..."
echo ""

SESSION_SECRET=$(openssl rand -hex 32)

cat > "$ENV_PATH" <<EOF
# TitipDong secrets — DO NOT COMMIT, DO NOT DELETE.
# Generated $(date -u +"%Y-%m-%dT%H:%M:%SZ")
# Backup these values somewhere safe (password manager).

# Session cookie signing key. If changed, all logged-in users get logged out.
SESSION_SECRET=$SESSION_SECRET

# Connect to your Postgres app catalog on the TrueNAS host.
# Replace __TRUENAS_IP__ with the LAN IP of this TrueNAS box.
DATABASE_URL=postgres://postgres:__POSTGRES_PASSWORD__@__TRUENAS_IP__:5432/$DB_NAME?sslmode=disable

# First-boot admin (leave both empty to inject admin via SQL instead).
ADMIN_EMAIL=
ADMIN_PASSWORD=

# Public HTTPS URL via Cloudflare Tunnel.
BASE_URL=https://titipdong.parifi.dev

# OpenAI key to enable receipt/struk scan (empty = feature hidden).
# Get one at https://platform.openai.com/api-keys
OPENAI_API_KEY=
EOF

chmod 600 "$ENV_PATH"
echo -e "${GREEN}Secrets file created: $ENV_PATH${NC}"
echo ""
echo -e "${YELLOW}IMPORTANT — action items:${NC}"
echo ""
echo "1. Edit the file to fill in remaining values:"
echo "      sudo nano $ENV_PATH"
echo ""
echo "   - Replace __POSTGRES_PASSWORD__ with your real Postgres password"
echo "   - Replace __TRUENAS_IP__ with this host's LAN IP"
echo "   - Set ADMIN_EMAIL/ADMIN_PASSWORD, OR leave empty to inject via SQL"
echo "   - Set OPENAI_API_KEY (or leave empty)"
echo ""
echo "2. BACK UP these secrets to a password manager now:"
echo "      sudo cat $ENV_PATH"
echo "      # copy to 1Password/Bitwarden, then clear terminal history"
echo ""
echo "3. Make sure your Postgres has a database named '$DB_NAME':"
echo "      PGPASSWORD='<postgres-pw>' psql -h <truenas-ip> -U postgres -c \"CREATE DATABASE $DB_NAME;\""
echo ""
echo "4. Deploy the app via TrueNAS → Apps → Custom App → 'Install via YAML',"
echo "   using docker-compose.truenas.yml from the repo."
echo ""
echo -e "${GREEN}Done. Secrets persist across app edits/redeploys.${NC}"
