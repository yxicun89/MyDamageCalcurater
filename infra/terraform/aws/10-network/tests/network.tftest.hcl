# terraform test(mock_provider)で、クラウドにもエミュレータにも触れずに計画を確かめる。make tf-test
mock_provider "aws" {}

variables {
  owner = "test-owner"
}

run "defaults_have_no_nat_and_no_inbound" {
  command = plan

  assert {
    condition     = length(aws_nat_gateway.this) == 0 && length(aws_eip.nat) == 0 && length(aws_route.private_nat) == 0
    error_message = "既定(enable_nat = false)で NAT ゲートウェイ・EIP・NAT への経路を作ってはいけない(時間課金)"
  }

  assert {
    condition     = aws_route.public_internet.destination_cidr_block == "0.0.0.0/0"
    error_message = "VM 用サブネットはインターネットゲートウェイから外へ出られること"
  }

  assert {
    condition     = aws_vpc.this.cidr_block == "10.20.0.0/16" && aws_subnet.public.cidr_block == "10.20.0.0/24"
    error_message = "既定の CIDR が変わった"
  }

  assert {
    condition     = aws_vpc_endpoint.s3.service_name == "com.amazonaws.ap-northeast-1.s3" && aws_vpc_endpoint.s3.vpc_endpoint_type == "Gateway"
    error_message = "S3 のエンドポイントは同じリージョンのゲートウェイ型(無料)にする"
  }
}

run "nat_is_created_only_when_enabled" {
  command = plan

  variables {
    enable_nat = true
  }

  assert {
    condition     = length(aws_nat_gateway.this) == 1 && length(aws_route.private_nat) == 1
    error_message = "enable_nat = true で NAT ゲートウェイと経路が 1 つずつできること"
  }
}

run "rejects_empty_owner" {
  command = plan

  variables {
    owner = " "
  }

  expect_failures = [var.owner]
}

run "rejects_invalid_cidr" {
  command = plan

  variables {
    vpc_cidr = "10.20.0.0"
  }

  expect_failures = [var.vpc_cidr]
}
