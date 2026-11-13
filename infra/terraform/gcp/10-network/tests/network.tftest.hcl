# terraform test(mock_provider)で、クラウドに触れずに計画を確かめる。GCP には VPC を扱える公式エミュレータが
# 無いため、GCP 側の確認はこのモックだけ(ADR-0815)。make tf-test
mock_provider "google" {}

variables {
  project_id = "test-project"
  owner      = "test-owner"
}

run "defaults_have_no_nat_and_custom_subnets" {
  command = plan

  assert {
    condition     = length(google_compute_router.nat) == 0 && length(google_compute_router_nat.this) == 0
    error_message = "既定(enable_nat = false)で Cloud NAT を作ってはいけない(時間課金)"
  }

  assert {
    condition     = google_compute_network.this.auto_create_subnetworks == false
    error_message = "全リージョンにサブネットを作る自動モードにしない(受信を既定で拒否するカスタムモード)"
  }

  assert {
    condition     = google_compute_subnetwork.vm.region == "asia-northeast1" && google_compute_subnetwork.vm.ip_cidr_range == "10.20.0.0/24"
    error_message = "既定のリージョン・CIDR が変わった"
  }
}

run "nat_is_created_only_when_enabled" {
  command = plan

  variables {
    enable_nat = true
  }

  assert {
    condition     = length(google_compute_router_nat.this) == 1
    error_message = "enable_nat = true で Cloud NAT が 1 つできること"
  }
}

run "rejects_label_unsafe_owner" {
  command = plan

  variables {
    owner = "Example Owner"
  }

  expect_failures = [var.owner]
}
