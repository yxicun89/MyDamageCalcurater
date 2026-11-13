variable "region" {
  description = "AWS のリージョン(cloud-design.md §3.1: 東京)"
  type        = string
  default     = "ap-northeast-1"
}

variable "env" {
  description = "Env タグの値"
  type        = string
  default     = "dev"
}

variable "owner" {
  description = "Owner タグの値。固有値は Git に入れず *.tfvars で渡す(example.tfvars は架空値)"
  type        = string

  validation {
    condition     = length(trimspace(var.owner)) > 0
    error_message = "owner は空にできない。"
  }
}

variable "vpc_cidr" {
  description = "VPC の CIDR"
  type        = string
  default     = "10.20.0.0/16"

  validation {
    condition     = can(cidrhost(var.vpc_cidr, 0))
    error_message = "vpc_cidr は CIDR 表記にする。"
  }
}

variable "public_subnet_cidr" {
  description = "VM を置くサブネットの CIDR(NAT を使わず、インターネットゲートウェイから外へ出る)"
  type        = string
  default     = "10.20.0.0/24"

  validation {
    condition     = can(cidrhost(var.public_subnet_cidr, 0))
    error_message = "public_subnet_cidr は CIDR 表記にする。"
  }
}

variable "private_subnet_cidr" {
  description = "外へ出るのに NAT が要るサブネットの CIDR(enable_nat = true のときだけ外へ出られる)"
  type        = string
  default     = "10.20.1.0/24"

  validation {
    condition     = can(cidrhost(var.private_subnet_cidr, 0))
    error_message = "private_subnet_cidr は CIDR 表記にする。"
  }
}

variable "enable_nat" {
  description = "NAT ゲートウェイを作るか。時間課金が高いので既定は false(cloud-design.md §5.1・§5.4 の 2)"
  type        = bool
  default     = false
}
