output "network_id" {
  value = google_compute_network.this.id
}

output "subnetwork_id" {
  description = "VM を置くサブネット"
  value       = google_compute_subnetwork.vm.id
}
