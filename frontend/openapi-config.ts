import type { ConfigFile } from '@rtk-query/codegen-openapi'

const config: ConfigFile = {
  schemaFile: '../backend/internal/api/spec.yaml',
  apiFile: './src/store/emptyApi.ts',
  apiImport: 'emptySplitApi',
  outputFile: './src/store/generatedApi.ts',
  exportName: 'api',
  hooks: true,
  filterEndpoints: [/^(?!healthz$)/],
  endpointOverrides: [
    { pattern: 'loginUser', type: 'mutation' },
    { pattern: 'registerUser', type: 'mutation' },
    { pattern: 'launchHypothesis', type: 'mutation' },
    { pattern: 'stopHypothesis', type: 'mutation' },
    { pattern: 'startProductAnalysis', type: 'mutation' },
    { pattern: 'uploadProductFile', type: 'mutation' },
    { pattern: 'runProductFeedbackSurvey', type: 'mutation' },
    {
      pattern: /.*/,
      parameterFilter: (_name, parameter) => parameter.in !== 'header',
    },
  ],
}

export default config
