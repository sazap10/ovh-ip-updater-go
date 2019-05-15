workflow "Build and tag" {
  resolves = [
    "Push image"
  ]
  on = "push"
}

action "Lint code" {
  uses = "actions/docker/cli@8cdf801b322af5f369e00d85e9cf3a7122f49108"
  args = "build --target ci ."
}

action "Build image" {
  uses = "actions/docker/cli@8cdf801b322af5f369e00d85e9cf3a7122f49108"
  args = "build -t ovh-ip-updater-go ."
}

action "Tag image" {
  uses = "actions/docker/tag@8cdf801b322af5f369e00d85e9cf3a7122f49108"
  needs = [
    "Build image",
    "Lint code",
  ]
  args = "ovh-ip-updater-go sazap10/ovh-ip-updater-go"
}

action "Push image" {
  uses = "actions/docker/cli@8cdf801b322af5f369e00d85e9cf3a7122f49108"
  needs = ["Docker login"]
  args = "push sazap10/ovh-ip-updater-go:$IMAGE_SHA"
}

action "Docker login" {
  uses = "actions/docker/login@8cdf801b322af5f369e00d85e9cf3a7122f49108"
  needs = ["Tag image"]
  secrets = ["DOCKER_USERNAME", "DOCKER_PASSWORD"]
}

workflow "Build and push on tag" {
  on = "push"
  resolves = ["Push image with ref"]
}

action "Only run on tag" {
  uses = "actions/bin/filter@3c0b4f0e63ea54ea5df2914b4fabf383368cd0da"
  needs = ["Docker login"]
  args = "tag"
}

action "Push image with ref" {
  uses = "actions/docker/cli@8cdf801b322af5f369e00d85e9cf3a7122f49108"
  needs = ["Only run on tag"]
  args = "push sazap10/ovh-ip-updater-go:$IMAGE_REF"
}

workflow "Build and push on master" {
  on = "push"
  resolves = ["Push image to latest"]
}

action "Filter master" {
  uses = "actions/bin/filter@3c0b4f0e63ea54ea5df2914b4fabf383368cd0da"
  needs = ["Docker login"]
  args = "branch master"
}

action "Push image to latest" {
  uses = "actions/docker/cli@8cdf801b322af5f369e00d85e9cf3a7122f49108"
  needs = ["Filter master"]
  args = "push sazap10/ovh-ip-updater-go:latest"
}
