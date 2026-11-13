# 段階 1・2 の VM を置くネットワーク(cloud-design.md §5.1 の 10-network)。
# 公開の入口は作らない(受信を許す SG ルールが無い。到達は Tailscale。§0・§2.6)。

resource "aws_vpc" "this" {
  cidr_block           = var.vpc_cidr
  enable_dns_support   = true
  enable_dns_hostnames = true

  tags = { Name = "pokecalc-${var.env}" }
}

resource "aws_internet_gateway" "this" {
  vpc_id = aws_vpc.this.id

  tags = { Name = "pokecalc-${var.env}" }
}

# --- VM 用(インターネットゲートウェイで外へ出る。受信は SG で閉じる) ---

resource "aws_subnet" "public" {
  vpc_id                  = aws_vpc.this.id
  cidr_block              = var.public_subnet_cidr
  map_public_ip_on_launch = true

  tags = { Name = "pokecalc-${var.env}-public" }
}

resource "aws_route_table" "public" {
  vpc_id = aws_vpc.this.id

  tags = { Name = "pokecalc-${var.env}-public" }
}

resource "aws_route" "public_internet" {
  route_table_id         = aws_route_table.public.id
  destination_cidr_block = "0.0.0.0/0"
  gateway_id             = aws_internet_gateway.this.id
}

resource "aws_route_table_association" "public" {
  subnet_id      = aws_subnet.public.id
  route_table_id = aws_route_table.public.id
}

# --- NAT 経由で外へ出る側(enable_nat = false の間は外へ出られない) ---

resource "aws_subnet" "private" {
  vpc_id     = aws_vpc.this.id
  cidr_block = var.private_subnet_cidr

  tags = { Name = "pokecalc-${var.env}-private" }
}

resource "aws_route_table" "private" {
  vpc_id = aws_vpc.this.id

  tags = { Name = "pokecalc-${var.env}-private" }
}

resource "aws_route_table_association" "private" {
  subnet_id      = aws_subnet.private.id
  route_table_id = aws_route_table.private.id
}

resource "aws_eip" "nat" {
  count  = var.enable_nat ? 1 : 0
  domain = "vpc"

  tags = { Name = "pokecalc-${var.env}-nat" }
}

resource "aws_nat_gateway" "this" {
  count         = var.enable_nat ? 1 : 0
  allocation_id = aws_eip.nat[0].id
  subnet_id     = aws_subnet.public.id

  tags = { Name = "pokecalc-${var.env}" }

  depends_on = [aws_internet_gateway.this]
}

resource "aws_route" "private_nat" {
  count                  = var.enable_nat ? 1 : 0
  route_table_id         = aws_route_table.private.id
  destination_cidr_block = "0.0.0.0/0"
  nat_gateway_id         = aws_nat_gateway.this[0].id
}

# --- VM の SG: 受信ルールを作らない(公開の入口を作らない)。送信だけ許す ---

resource "aws_security_group" "vm" {
  name        = "pokecalc-${var.env}-vm"
  description = "pokecalc VM: no inbound rules, outbound only"
  vpc_id      = aws_vpc.this.id

  tags = { Name = "pokecalc-${var.env}-vm" }
}

resource "aws_vpc_security_group_egress_rule" "vm_all" {
  security_group_id = aws_security_group.vm.id
  description       = "all outbound (k3s, Tailscale, image pulls)"
  ip_protocol       = "-1"
  cidr_ipv4         = "0.0.0.0/0"
}

# --- S3 へのゲートウェイ型エンドポイント(無料。バックアップの書き込みを VPC 内で済ませる) ---

resource "aws_vpc_endpoint" "s3" {
  vpc_id            = aws_vpc.this.id
  service_name      = "com.amazonaws.${var.region}.s3"
  vpc_endpoint_type = "Gateway"
  route_table_ids   = [aws_route_table.public.id, aws_route_table.private.id]

  tags = { Name = "pokecalc-${var.env}-s3" }
}
