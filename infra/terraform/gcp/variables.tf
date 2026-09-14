variable "project_id" {
  description = "GCP project that will own the TelemetryForge GKE foundation."
  type        = string
}

variable "name" {
  description = "Resource name prefix."
  type        = string
  default     = "telemetryforge"
}

variable "region" {
  description = "GCP region."
  type        = string
  default     = "us-central1"
}

variable "subnet_cidr" {
  description = "GKE subnet CIDR."
  type        = string
  default     = "10.52.0.0/20"
}

variable "node_machine_type" {
  description = "GKE node machine type."
  type        = string
  default     = "e2-standard-4"
}

variable "node_count" {
  description = "Nodes per regional location."
  type        = number
  default     = 1
}
