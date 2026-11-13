# 段階 1・2 の VM を置くネットワーク(cloud-design.md §5.1 の 10-network。AWS 版と同じ分け方)。
# カスタムモードの VPC は受信を既定で拒否する。受信を許すファイアウォールルールは作らない(到達は Tailscale。§0・§2.6)。

resource "google_compute_network" "this" {
  name                    = "pokecalc-${var.env}"
  auto_create_subnetworks = false
  routing_mode            = "REGIONAL"
}

resource "google_compute_subnetwork" "vm" {
  name                     = "pokecalc-${var.env}-vm"
  network                  = google_compute_network.this.id
  region                   = var.region
  ip_cidr_range            = var.subnet_cidr
  private_ip_google_access = true
}

# --- Cloud NAT(enable_nat = false の間は作らない) ---

resource "google_compute_router" "nat" {
  count   = var.enable_nat ? 1 : 0
  name    = "pokecalc-${var.env}-nat"
  network = google_compute_network.this.id
  region  = var.region
}

resource "google_compute_router_nat" "this" {
  count                              = var.enable_nat ? 1 : 0
  name                               = "pokecalc-${var.env}"
  router                             = google_compute_router.nat[0].name
  region                             = var.region
  nat_ip_allocate_option             = "AUTO_ONLY"
  source_subnetwork_ip_ranges_to_nat = "ALL_SUBNETWORKS_ALL_IP_RANGES"
}
