workflow "Build and deploy on push" {
  on = "push"
  resolves = ["Lint code"]
}

action "Lint code" {
  uses = "actions/docker/cli@8cdf801b322af5f369e00d85e9cf3a7122f49108"
  args = "build --target ci ."
}
