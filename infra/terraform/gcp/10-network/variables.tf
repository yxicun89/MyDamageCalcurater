variable "project_id" {
  description = "GCP のプロジェクト ID。固有値は Git に入れず *.tfvars で渡す(example.tfvars は架空値)"
  type        = string

  validation {
    condition     = length(trimspace(var.project_id)) > 0
    error_message = "project_id は空にできない。"
  }
}

variable "region" {
  description = "GCP のリージョン(cloud-design.md §3.1: 東京)"
  type        = string
  default     = "asia-northeast1"
}

variable "env" {
  description = "env ラベルの値"
  type        = string
  default     = "dev"
}

variable "owner" {
  description = "owner ラベルの値(小文字・数字・_・- だけ)。固有値は *.tfvars で渡す"
  type        = string

  validation {
    condition     = can(regex("^[a-z0-9_-]{1,63}$", var.owner))
    error_message = "owner は GCP のラベルの値として使える文字(小文字・数字・_・-、63 文字まで)にする。"
  }
}

variable "subnet_cidr" {
  description = "VM を置くサブネットの CIDR"
  type        = string
  default     = "10.20.0.0/24"

  validation {
    condition     = can(cidrhost(var.subnet_cidr, 0))
    error_message = "subnet_cidr は CIDR 表記にする。"
  }
}

variable "enable_nat" {
  description = "Cloud NAT を作るか。時間課金なので既定は false(cloud-design.md §5.1・§5.4 の 2)"
  type        = bool
  default     = false
}
