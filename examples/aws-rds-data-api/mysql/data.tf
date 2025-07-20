# Read outputs from the infrastructure stack
data "terraform_remote_state" "infrastructure" {
  backend = "local"

  config = {
    path = "${var.infrastructure_stack_path}/terraform.tfstate"
  }
}