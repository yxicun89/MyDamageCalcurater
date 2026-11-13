terraform {
  # 版は厳密に固定する(docs/plan/cloud-design.md §5.1)。上げるときは .terraform.lock.hcl も make tf-lock で作り直す
  required_version = "= 1.17.2"

  required_providers {
    aws = {
      source  = "hashicorp/aws"
      version = "= 6.73.0"
    }
  }

  # backend(state の置き場)はまだ決めていない(cloud-design.md §8 Q7)。決まるまではローカル state
}

provider "aws" {
  region = var.region

  # 全リソースに付けるタグ(cloud-design.md §5.1)。消し忘れの棚卸しに使う(§5.3)
  default_tags {
    tags = {
      Project   = "pokecalc"
      Env       = var.env
      ManagedBy = "terraform"
      Owner     = var.owner
      Ephemeral = "false"
    }
  }
}
