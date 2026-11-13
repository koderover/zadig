# Efficiency diagnosis

## Applicable scenarios

The user wants to analyze build, test, deployment and release quality and efficiency for explicitly specified projects, identify bottlenecks and receive improvement suggestions.

## Required information

One or more explicit project names or keys, plus an analysis time range. Default to the last thirty days, with at most ten projects and ninety days per request. Ask for projects if none are specified; do not scan all projects.

## Steps

1. Use the efficiency diagnosis tool with system-level efficiency insight permissions. Per-project viewing permission is not required.
2. Explicitly submit the project list and start and end times. Reuse the report returned by the existing efficiency analysis interface and its model; do not create a separate statistics algorithm.
3. If project names match multiple projects, show the candidate identifiers returned by the original interface and ask the user to choose. Do not expand the scope to all projects.
4. Explain bottlenecks, key metrics and actionable suggestions using the original report. Distinguish facts, report conclusions and further inferences.
5. Do not repeatedly request the same report within one run. Accurately explain timeouts, failures or missing valid reports without automatically retrying.

## Clarification

Call the Skill's business tool even when parameters are incomplete, then ask about the returned `missing_fields`. Continue the same task with its `skill_execution_id` and submit only parameters added or changed in the current turn. The backend restores saved parameters; a new page must not replace the original target.

Ask about missing or ambiguous projects. If the project count or time range exceeds the limit, or only one time boundary is supplied, request a complete, bounded scope. Do not substitute the current page's project for an analysis target the user has not explicitly specified.

## Output requirements

Use the system language selected for this generation to identify the specified projects, start and end times, report source, quality and efficiency conclusions, and improvement suggestions. Use only the original report body. Do not show internal prompts, input data or credentials, or count estimated Token usage as actual consumption.

## Completion criteria

Count completion only after obtaining a valid original report and answering the user. Do not claim success for a failed or empty report, or automatically modify build or workflow configuration.
