variable "region" {
  type        = string
  description = "AWS Region"
}

variable "namespace" {
  type        = string
  description = "Namespace for app"
}

variable "stage" {
  type        = string
  description = "Environment level (prod, stage, qa, dev)"
}

variable "name" {
  type        = string
  description = "Name of the app"
}

variable "attributes" {
  type        = list(string)
  description = "Attributes for the app id"
  default     = []
}

variable "delimiter" {
  type        = string
  description = "Delimiter for app id"
  default     = "-"
}

variable "tags" {
  type        = map(string)
  description = "Additional tags (_e.g._ { BusinessUnit : ABC })"
  default     = {}
}

variable "target_group_port" {
  type        = number
  description = "Target group port"
}

variable "target_group_protocol" {
  type        = string
  description = "Target group protocol (HTTP)"
}

variable "target_group_target_type" {
  type        = string
  description = "Target group target type (ip, instance)"
}

variable "health_check_path" {
  type        = string
  description = "Health check path for target group"
}

variable "health_check_timeout" {
  type        = number
  description = "Health check timeout for target group"
  default     = 10
}

variable "health_check_healthy_threshold" {
  type        = number
  description = "Health check threshold to determine if working"
  default     = 2
}

variable "health_check_unhealthy_threshold" {
  type        = number
  description = "Health check threshold to determine if failing"
  default     = 2
}

variable "health_check_interval" {
  type        = number
  description = "Interval between health check in seconds"
  default     = 15
}

variable "health_check_matcher" {
  type        = string
  description = "The HTTP response codes to indicate a healthy check"
  default     = "200-399"
}

variable "load_balancer_listener_arn" {
  type        = string
  description = "Load balancer listener arn"
}

variable "load_balancer_listener_paths" {
  type        = list(string)
  description = "Conditional paths for load balancer listener forwarding"
}

variable "container_image" {
  type        = string
  description = "The image used to start the container. Images in the Docker Hub registry available by default"
}

variable "container_memory" {
  type        = number
  description = "The amount of memory (in MiB) to allow the container to use. This is a hard limit, if the container attempts to exceed the container_memory, the container is killed. This field is optional for Fargate launch type and the total amount of container_memory of all containers in a task will need to be lower than the task memory value"
}

variable "container_memory_reservation" {
  type        = number
  description = "The amount of memory (in MiB) to reserve for the container. If container needs to exceed this threshold, it can do so up to the set container_memory hard limit"
}

variable "container_port_mappings" {
  type = list(object({
    containerPort = number
    hostPort      = number
    protocol      = string
  }))

  description = "The port mappings to configure for the container. This is a list of maps. Each map should contain \"containerPort\", \"hostPort\", and \"protocol\", where \"protocol\" is one of \"tcp\" or \"udp\". If using containers in a task with the awsvpc or host network mode, the hostPort can either be left blank or set to the same value as the containerPort"
}

variable "container_port" {
  type        = number
  description = "Container port"
}

variable "container_cpu" {
  type        = number
  description = "The number of cpu units to reserve for the container. This is optional for tasks using Fargate launch type and the total amount of container_cpu of all containers in a task will need to be lower than the task-level cpu value"
}

variable "container_essential" {
  type        = bool
  description = "Determines whether all other containers in a task are stopped, if this container fails or stops for any reason. Due to how Terraform type casts booleans in json it is required to double quote this value"
}

variable "container_environment" {
  type = list(object({
    name  = string
    value = string
  }))
  description = "The environment variables to pass to the container. This is a list of maps"
}

variable "container_secrets" {
  type = list(object({
    name      = string
    valueFrom = string
  }))
  description = "The secrets to pass to the container. This is a list of maps"
  default     = null
}

variable "container_readonly_root_filesystem" {
  type        = bool
  description = "Determines whether a container is given read-only access to its root filesystem. Due to how Terraform type casts booleans in json it is required to double quote this value"
}

variable "vpc_default_security_group_id" {
  type        = string
  description = "ID of the default VPC security group"
}

variable "ecs_cluster_arn" {
  type        = string
  description = "ECS Cluster ARN"
}

variable "ecs_launch_type" {
  type        = string
  description = "ECS launch type"
}

variable "vpc_id" {
  type        = string
  description = "VPC ID"
}

variable "private_subnet_ids" {
  type = list(string)
}

variable "ignore_changes_task_definition" {
  type        = bool
  description = "Ignore changes to task definition"
}

variable "network_mode" {
  type        = string
  description = "The network mode to use for the task. This is required to be `awsvpc` for `FARGATE` `launch_type`"
}

variable "assign_public_ip" {
  type        = bool
  description = "Assign a public IP address to the ENI (Fargate launch type only). Valid values are `true` or `false`. Default `false`"
}

variable "propagate_tags" {
  type        = string
  description = "Specifies whether to propagate the tags from the task definition or the service to the tasks. The valid values are SERVICE and TASK_DEFINITION"
}

variable "deployment_minimum_healthy_percent" {
  type        = number
  description = "The lower limit (as a percentage of `desired_count`) of the number of tasks that must remain running and healthy in a service during a deployment"
}

variable "deployment_maximum_percent" {
  type        = number
  description = "The upper limit of the number of tasks (as a percentage of `desired_count`) that can be running in a service during a deployment"
}

variable "deployment_controller_type" {
  type        = string
  description = "Type of deployment controller. Valid values are `CODE_DEPLOY` and `ECS`"
}

variable "desired_count" {
  type        = number
  description = "The number of instances of the task definition to place and keep running"
}

variable "task_memory" {
  type        = number
  description = "The amount of memory (in MiB) used by the task. If using Fargate launch type `task_memory` must match supported cpu value (https://docs.aws.amazon.com/AmazonECS/latest/developerguide/task_definition_parameters.html#task_size)"
}

variable "task_cpu" {
  type        = number
  description = "The number of CPU units used by the task. If using `FARGATE` launch type `task_cpu` must match supported memory values (https://docs.aws.amazon.com/AmazonECS/latest/developerguide/task_definition_parameters.html#task_size)"
}

variable "task_role_policy" {
  type        = string
  description = "Task role policy"
}

variable "task_exec_role_policy" {
  type        = string
  description = "Task exec role policy"
}

variable "db_enabled" {
  type        = bool
  description = "Enable or disable DB access"
  default     = false
}

variable "db_port" {
  type        = number
  description = "Database access port"
  default     = 0
}

variable "db_security_group_id" {
  type        = string
  description = "Database security group ID"
  default     = ""
}

variable "redis_enabled" {
  type        = bool
  description = "Enable redis for service"
  default     = false
}

variable "redis_port" {
  type        = number
  description = "Redis port"
  default     = 6379
}

variable "redis_security_group_id" {
  type        = string
  description = "Redis security group id"
}
