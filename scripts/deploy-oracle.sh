#!/bin/bash
# Deploy Folio to Oracle Free Tier ARM VM
# Prerequisites: Ubuntu 22.04 ARM instance, SSH access
#
# This script is meant to be run ON the Oracle VM after cloning the repo.

set -euo pipefail

echo "=== Folio Oracle Free Tier Deployment ==="

# 1. Install Docker
if ! command -v docker &> /dev/null; then
    echo "[1/6] Installing Docker..."
    curl -fsSL https://get.docker.com | sh
    sudo usermod -aG docker $USER
    echo "  Docker installed. You may need to log out and back in."
    echo "  Then re-run this script."
    exit 0
else
    echo "[1/6] Docker already installed"
fi

# 2. Install Docker Compose plugin
if ! docker compose version &> /dev/null; then
    echo "[2/6] Installing Docker Compose plugin..."
    sudo apt-get update
    sudo apt-get install -y docker-compose-plugin
else
    echo "[2/6] Docker Compose already installed"
fi

# 3. Setup DuckDNS
echo "[3/6] DuckDNS Setup"
echo "  1. Go to https://www.duckdns.org and create a free account"
echo "  2. Create a subdomain (e.g., folio.duckdns.org)"
echo "  3. Point it to your Oracle VM's public IP"
read -p "  Enter your DuckDNS domain (e.g., folio.duckdns.org): " DOMAIN
read -p "  Enter your DuckDNS token: " DUCKDNS_TOKEN

# Setup DuckDNS auto-update cron
mkdir -p ~/duckdns
cat > ~/duckdns/duck.sh << DUCKEOF
#!/bin/bash
echo url="https://www.duckdns.org/update?domains=$(echo $DOMAIN | sed 's/.duckdns.org//')&token=${DUCKDNS_TOKEN}&ip=" | curl -k -o ~/duckdns/duck.log -K -
DUCKEOF
chmod 700 ~/duckdns/duck.sh
(crontab -l 2>/dev/null; echo "*/5 * * * * ~/duckdns/duck.sh >/dev/null 2>&1") | crontab -
~/duckdns/duck.sh
echo "  DuckDNS configured and cron set"

# 4. Configure .env
echo "[4/6] Configuring environment..."
cd "$(dirname "$0")/.."

if [ ! -f .env ]; then
    cp .env.example .env
fi

# Set domain in .env
sed -i "s|DOMAIN=localhost|DOMAIN=${DOMAIN}|" .env

echo "  .env configured with domain: $DOMAIN"

# 5. Open firewall ports
echo "[5/6] Configuring firewall..."
sudo iptables -I INPUT -p tcp --dport 80 -j ACCEPT
sudo iptables -I INPUT -p tcp --dport 443 -j ACCEPT
sudo netfilter-persistent save 2>/dev/null || true
echo "  Ports 80 and 443 opened"
echo "  IMPORTANT: Also open ports 80 and 443 in Oracle Cloud Console:"
echo "    VCN > Security Lists > Ingress Rules"

# 6. Start everything
echo "[6/6] Starting Folio..."
bash "$(dirname "$0")/setup.sh"

echo ""
echo "======================================="
echo "  Folio is live at https://${DOMAIN}"
echo "  Login: admin@folio.local / admin"
echo "  CHANGE THE ADMIN PASSWORD!"
echo "======================================="
