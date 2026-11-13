# Service failure analysis

## Applicable scenarios

The user wants to view service logs, status or events, investigate service problems in an environment, or explicitly requests a service restart or an environment version rollback. Viewing logs does not require a service failure or an inspection of the entire environment. Use this Skill even when the current page is an environment page.

## Required information

The current `goal` (`diagnose`, `restart_service` or `rollback_environment`), project, environment and service name. Identify the workload, Pod and container when reading additional logs. Rollbacks require an explicit target version.

## Steps

1. The first call must specify `goal` according to the user's explicit intent. Use `diagnose` for requests to view logs, status or events or analyze a problem, without asking for a repair action. When the user requests a repair but the action is unclear, ask for clarification; do not silently complete a diagnosis instead. Read actual status, workloads, events, bounded logs and deployment history using the service diagnosis tool.
2. Distinguish startup failures, image pull failures, probe failures, insufficient resources and configuration problems based on evidence. Explain the certainty of the diagnosis.
3. Default to read-only diagnosis and repair suggestions. Prepare a concrete repair plan only when the user explicitly requests a restart or rollback.
4. Repairs require authorization through a confirmation card, followed by programmatic permission checks and execution. Chat confirmation cannot replace the confirmation API. Verify unknown results before submitting anything again.

## Clarification

Call the Skill's business tool even when parameters are incomplete, then ask about the returned `missing_fields`. Continue the same task with its `skill_execution_id` and submit only parameters added or changed in the current turn. The backend restores saved parameters; a new page must not replace the original target.

If the project, environment or service is unclear, present the necessary candidates for selection. Ask for an unclear rollback target instead of selecting a version. Do not downgrade a repair goal to diagnosis within the same execution; changing the repair action starts a new task.

## Output requirements

Use the system language selected for this generation to describe affected resources, evidence, causes, suggestions and verification steps. Clearly distinguish read-only diagnosis, operations awaiting confirmation, completed operations and results awaiting verification. Do not expose credentials.

## Completion criteria

Only `goal=diagnose` can complete after collecting evidence and providing a valid analysis. Repair goals still require a confirmation plan after evidence collection; a diagnosis response alone does not count as success. Requested repairs must be judged by business results verified by the program. Preparing a plan or submitting a request does not prove repair success.
