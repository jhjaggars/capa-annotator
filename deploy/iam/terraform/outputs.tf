# Outputs for CAPA Annotator IAM Resources

output "role_arn" {
  description = "ARN of the IAM role for CAPA Annotator (use this in ServiceAccount annotation)"
  value       = aws_iam_role.capa_annotator.arn
}

output "role_name" {
  description = "Name of the IAM role"
  value       = aws_iam_role.capa_annotator.name
}

output "policy_arn" {
  description = "ARN of the IAM policy"
  value       = aws_iam_policy.capa_annotator.arn
}

output "policy_name" {
  description = "Name of the IAM policy"
  value       = aws_iam_policy.capa_annotator.name
}

output "oidc_provider_arn" {
  description = "ARN of the OIDC provider used for trust relationship"
  value       = "arn:aws:iam::${data.aws_caller_identity.current.account_id}:oidc-provider/${var.oidc_provider_url}"
}
