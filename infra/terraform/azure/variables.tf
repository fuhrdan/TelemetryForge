variable "name" {
  description = "Resource name prefix."
  type        = string
  default     = "telemetryforge"
}

variable "location" {
  description = "Azure region."
  type        = string
  default     = "West US 2"
}

variable "vnet_cidr" {
  description = "Virtual network CIDR."
  type        = list(string)
  default     = ["10.62.0.0/16"]
}

variable "subnet_cidr" {
  description = "AKS subnet CIDR."
  type        = list(string)
  default     = ["10.62.0.0/20"]
}

variable "node_vm_size" {
  description = "AKS system node VM size."
  type        = string
  default     = "Standard_D4s_v5"
}

variable "node_count" {
  description = "AKS system node count."
  type        = number
  default     = 3
}
