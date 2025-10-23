# Variables for CAPA Annotator IAM Role Configuration

variable "role_name" {
  description = "Name of the IAM role for CAPA Annotator"
  type        = string
  default     = "capa-annotator-role"
}

variable "policy_name" {
  description = "Name of the IAM policy for CAPA Annotator"
  type        = string
  default     = "capa-annotator-policy"
}

variable "role_path" {
  description = "Path for the IAM role"
  type        = string
  default     = "/"
}

variable "policy_path" {
  description = "Path for the IAM policy"
  type        = string
  default     = "/"
}

variable "oidc_provider_url" {
  description = "OIDC provider URL without https:// prefix (e.g., oidc.eks.us-west-2.amazonaws.com/id/EXAMPLED539D4633E53DE1B71EXAMPLE)"
  type        = string
}

variable "namespace" {
  description = "Kubernetes namespace where CAPA Annotator is deployed"
  type        = string
  default     = "capa-annotator-system"
}

variable "service_account_name" {
  description = "Kubernetes service account name for CAPA Annotator"
  type        = string
  default     = "capa-annotator"
}

variable "tags" {
  description = "Additional tags to apply to IAM resources"
  type        = map(string)
  default = {
    ManagedBy = "Terraform"
    Component = "capa-annotator"
  }
}
