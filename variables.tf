variable "github_user" {
  type        = string
  description = "GitHub username for GHCR."
}

variable "github_token" {
  type        = string
  description = "GitHub PAT with write:packages scope."
  sensitive   = true
}
