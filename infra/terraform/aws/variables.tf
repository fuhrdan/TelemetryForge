variable "aws_region" {
  description = "AWS region for the TelemetryForge EKS foundation."
  type        = string
  default     = "us-west-2"
}

variable "name" {
  description = "Resource name prefix."
  type        = string
  default     = "telemetryforge"
}

variable "vpc_cidr" {
  description = "VPC CIDR."
  type        = string
  default     = "10.42.0.0/16"
}

variable "kubernetes_version" {
  description = "Newest Kubernetes minor currently in Amazon EKS standard support."
  type        = string
  default     = "1.36"
}

variable "node_instance_types" {
  description = "Managed node group instance types."
  type        = list(string)
  default     = ["m7i.large"]
}

variable "node_desired_size" {
  type    = number
  default = 3
}

variable "node_min_size" {
  type    = number
  default = 2
}

variable "node_max_size" {
  type    = number
  default = 6
}
