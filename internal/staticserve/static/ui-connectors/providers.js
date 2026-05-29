import { generateFriendlyConnectorName } from '/static/ui-connectors/name-utils.js';

export const providers = [
  {
    id: 'gemini',
    name: 'Google Gemini',
    apiKeyPlaceholder: 'gemini-xxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxx',
  },
  {
    id: 'deepseek',
    name: 'DeepSeek',
    apiKeyPlaceholder: 'sk-xxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxx',
    baseURLPlaceholder: 'https://api.deepseek.com/v1',
  },
  {
    id: 'openrouter',
    name: 'OpenRouter',
    apiKeyPlaceholder: 'sk-or-v1-xxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxx',
    baseURLPlaceholder: 'https://openrouter.ai/api/v1',
  },
  {
    id: 'ollama',
    name: 'Ollama',
    requiresBaseURL: true,
    baseURLPlaceholder: 'http://localhost:11434/ollama/api',
    apiKeyPlaceholder: 'Optional JWT token for authentication',
  },
  {
    id: 'openai',
    name: 'OpenAI',
    apiKeyPlaceholder: 'sk-xxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxx',
  },
  {
    id: 'claude',
    name: 'Anthropic Claude',
    apiKeyPlaceholder: 'sk-ant-api03-xxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxx',
  },
];

export function defaultForm() {
  const first = providers[0];
  return {
    id: '',
    provider_name: first.id,
    connector_name: generateFriendlyConnectorName(first.id, providers),
    api_key: '',
    base_url: '',
    selected_model: '',
  };
}
