variable "TAG" {
  default = "test"
}

variable "GITHUB_ACTIONS" {
  default = ""
}

variable "REGISTRY" {
  default = "ghcr.io/guilhem"
}

function "tags" {
  params = [name]
  result = "${REGISTRY}/${name}:${TAG}"
}

function "buildtag" {
  params = [name]
  result = "${REGISTRY}/${name}:${TAG}-build"
}

target "default" {
  context = "."

  cache-from = [
    "type=registry,ref=${buildtag("k8ssh")}",
    // notequal("", GITHUB_ACTIONS) ? "type=gha" : "",
  ]
  // ssh  = ["default"]
  tags = [tags("k8ssh")]
  cache-to = [
    "type=registry,ref=${buildtag("k8ssh")},mode=max",
    // notequal("", GITHUB_ACTIONS) ? "type=gha,mode=max" : "",
  ]
}
