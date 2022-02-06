region = "us-west-2"

namespace = "bot"

stage = "prod"

name = "nsm3"

attributes = []

delimiter = "-"

tags = {}

vpc_id = "vpc-07ac9bc4bf00dbe42"

vpc_default_security_group_id = "sg-037b3e9aa842c8b55"

ecs_cluster_arn = "arn:aws:ecs:us-west-2:358393647923:cluster/disc-prod-bots"

private_subnet_ids = [
  "subnet-0e352b0e20e5c652a",
  "subnet-049bfe3704ccafc4d",
]

load_balancer_listener_arn = "arn:aws:elasticloadbalancing:us-west-2:358393647923:listener/app/disc-prod-bots/0e5e0f8ad52a8964/67888df0e57a342c"

load_balancer_listener_paths = ["/nitrado-server-manager-v3/*"]

target_group_port = 80

target_group_protocol = "HTTP"

target_group_target_type = "ip"

health_check_path = "/nitrado-server-manager-v3/status"

health_check_timeout = 10

health_check_healthy_threshold = 2

health_check_unhealthy_threshold = 2

health_check_interval = 15

health_check_matcher = "200"

container_image = "358393647923.dkr.ecr.us-west-2.amazonaws.com/nitrado-server-manager-v3:prod"

container_memory = 1024

container_memory_reservation = 900

container_port_mappings = [
  {
    containerPort = 8080
    hostPort      = 8080
    protocol      = "tcp"
  }
]

container_port = 8080

container_cpu = 512

container_essential = true

container_environment = [
  {
    name  = "LISTENER_PORT",
    value = "8080"
  }
]

container_secrets = [
  {
    name      = "ENV_VARS"
    valueFrom = "arn:aws:secretsmanager:us-west-2:358393647923:secret:prod/nitrado-server-manager-v3-8buz6Q"
  }
]

container_readonly_root_filesystem = false

ecs_launch_type = "FARGATE"

ignore_changes_task_definition = true

network_mode = "awsvpc"

assign_public_ip = false

propagate_tags = "TASK_DEFINITION"

deployment_minimum_healthy_percent = 0

deployment_maximum_percent = 100

deployment_controller_type = "ECS"

desired_count = 1

task_memory = 1024

task_cpu = 512

task_role_policy = <<EOF
{
  "Version": "2012-10-17",
  "Statement": [
    {
      "Effect": "Allow",
      "Action": ["secretsmanager:GetSecretValue"],
      "Resource": "*"
    }
  ]
}
EOF

task_exec_role_policy = <<EOF
{
  "Version": "2012-10-17",
  "Statement": [
    {
      "Effect": "Allow",
      "Action": ["secretsmanager:GetSecretValue"],
      "Resource": "*"
    }
  ]
}
EOF

db_enabled = false

db_port = 0

db_security_group_id = ""

redis_enabled = true

redis_port = 6379

redis_security_group_id = "sg-0a95c3783e64fbbaf"
