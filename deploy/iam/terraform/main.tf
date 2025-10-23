# CAPA Annotator IAM Role for IRSA
# This Terraform configuration creates an IAM role with the minimal permissions
# required for the capa-annotator controller to function.

terraform {
  required_version = ">= 1.0"
  required_providers {
    aws = {
      source  = "hashicorp/aws"
      version = "~> 5.0"
    }
  }
}

# Data source to get the current AWS account ID
data "aws_caller_identity" "current" {}

# IAM Policy for CAPA Annotator
# Grants permissions to describe EC2 instance types and regions
resource "aws_iam_policy" "capa_annotator" {
  name        = var.policy_name
  description = "Policy for CAPA Annotator controller to query EC2 instance type information"
  path        = var.policy_path

  policy = jsonencode({
    Version = "2012-10-17"
    Statement = [
      {
        Effect = "Allow"
        Action = [
          "ec2:DescribeInstanceTypes",
          "ec2:DescribeRegions"
        ]
        Resource = "*"
      }
    ]
  })

  tags = merge(
    var.tags,
    {
      Name        = var.policy_name
      Description = "CAPA Annotator IAM Policy"
    }
  )
}

# IAM Role for CAPA Annotator with OIDC Trust Policy
resource "aws_iam_role" "capa_annotator" {
  name        = var.role_name
  description = "IAM role for CAPA Annotator controller using IRSA"
  path        = var.role_path

  assume_role_policy = jsonencode({
    Version = "2012-10-17"
    Statement = [
      {
        Effect = "Allow"
        Principal = {
          Federated = "arn:aws:iam::${data.aws_caller_identity.current.account_id}:oidc-provider/${var.oidc_provider_url}"
        }
        Action = "sts:AssumeRoleWithWebIdentity"
        Condition = {
          StringEquals = {
            "${var.oidc_provider_url}:sub" = "system:serviceaccount:${var.namespace}:${var.service_account_name}"
            "${var.oidc_provider_url}:aud" = "sts.amazonaws.com"
          }
        }
      }
    ]
  })

  tags = merge(
    var.tags,
    {
      Name        = var.role_name
      Description = "CAPA Annotator IAM Role"
    }
  )
}

# Attach the policy to the role
resource "aws_iam_role_policy_attachment" "capa_annotator" {
  role       = aws_iam_role.capa_annotator.name
  policy_arn = aws_iam_policy.capa_annotator.arn
}
