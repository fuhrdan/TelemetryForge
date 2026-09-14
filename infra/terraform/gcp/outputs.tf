output "cluster_name" {
  value = google_container_cluster.this.name
}

output "region" {
  value = var.region
}

output "kubectl_config_command" {
  value = "gcloud container clusters get-credentials ${google_container_cluster.this.name} --region ${var.region} --project ${var.project_id}"
}
