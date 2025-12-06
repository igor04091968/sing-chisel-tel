terraform {
  required_providers {
    docker = {
      source  = "kreuzwerker/docker"
      version = "~> 3.9.0"
    }
  }
}

provider "docker" {
  registry_auth {
    address  = "ghcr.io"
    username = var.github_user
    password = var.github_token
  }
}

resource "docker_image" "sing_chisel_tel_image" {
  name         = "ghcr.io/igor04091968/sing-chisel-tel:v1"
  build {
    context = "/mnt/usb_hdd1/Projects/sing-chisel-tel"
  }
}

resource "docker_container" "sing_chisel_tel_container" {
  image = docker_image.sing_chisel_tel_image.name
  name  = "sing-chisel-tel-container"
  ports {
    internal = 2095
    external = 2095
  }
  ports {
    internal = 2096
    external = 2096
  }
  volumes {
    host_path = "/mnt/usb_hdd1/Projects/sing-chisel-tel/db"
    container_path = "/app/db"
  }
  volumes {
    host_path = "/mnt/usb_hdd1/Projects/sing-chisel-tel/cert"
    container_path = "/app/cert"
  }
}
