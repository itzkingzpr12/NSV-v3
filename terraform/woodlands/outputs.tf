output "ecs_exec_role_policy_id" {
  description = "The ECS service role policy ID, in the form of `role_name:role_policy_name`"
  value       = module.service.ecs_exec_role_policy_id
}

output "ecs_exec_role_policy_name" {
  description = "ECS service role name"
  value       = module.service.ecs_exec_role_policy_name
}

output "service_name" {
  description = "ECS Service name"
  value       = module.service.service_name
}

output "service_role_arn" {
  description = "ECS Service role ARN"
  value       = module.service.service_role_arn
}

output "task_exec_role_name" {
  description = "ECS Task role name"
  value       = module.service.task_exec_role_name
}

output "task_exec_role_arn" {
  description = "ECS Task exec role ARN"
  value       = module.service.task_exec_role_arn
}

output "task_role_name" {
  description = "ECS Task role name"
  value       = module.service.task_role_name
}

output "task_role_arn" {
  description = "ECS Task role ARN"
  value       = module.service.task_role_arn
}

output "task_role_id" {
  description = "ECS Task role id"
  value       = module.service.task_role_id
}

output "service_security_group_id" {
  description = "Security Group ID of the ECS task"
  value       = module.service.service_security_group_id
}

output "task_definition_family" {
  description = "ECS task definition family"
  value       = module.service.task_definition_family
}

output "task_definition_revision" {
  description = "ECS task definition revision"
  value       = module.service.task_definition_revision
}
