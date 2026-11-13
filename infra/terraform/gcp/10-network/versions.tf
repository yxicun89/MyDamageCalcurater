terraform {
  # 版は厳密に固定する(docs/plan/cloud-design.md §5.1)。上げるときは .terraform.lock.hcl も make tf-lock で作り直す
  required_version = "= 1.17.2"

  required_providers {
    google = {
      source  = "hashicorp/google"
      version = "= 8.11.0"
    }
  }

  # backend(state の置き場)はまだ決めていない(cloud-design.md §8 Q7)。決まるまではローカル state
}

provider "google" {
  project = var.project_id
  region  = var.region

  # 全リソースに付けるラベル(cloud-design.md §5.1)。GCP のラベルは小文字だけ
  default_labels = {
    project    = "pokecalc"
    env        = var.env
    managed_by = "terraform"
    owner      = var.owner
    ephemeral  = "false"
  }
}
