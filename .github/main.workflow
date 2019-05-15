workflow "Build and deploy on push" {
  on = "push"
  resolves = [
    "Lint code",
    "Tag image",
  ]
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
  needs = ["Build image"]
  args = "ovh-ip-updater-go sazap10/ovh-ip-updater-go"
}
