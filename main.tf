terraform {
  required_providers {
    docker = {
      source  = "kreuzwerker/docker"
      version = "~> 3.9.0"
    }
  }
}

provider "docker" {}

resource "docker_image" "sing_chisel_tel_image" {
  name         = "sing-chisel-tel:v1"
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
}
