# Generator metrics and logs
## Metrics 
- total|success|error per node (node is llm call)
- retry count per node
- total|succes|error per pipeline
- somehow also tracking what we're retrying
error can happen if LLM returned something that we deem invalid and ask it to retry
## In runtime (e.g. in prod)
- count tokens in runtime per pipeline
- sum of time spent per pipeline
## Evaluator
- counting tokens (somehow, see how, explain here)
- sum of time spent per pipeline
can eval different models from different providers
