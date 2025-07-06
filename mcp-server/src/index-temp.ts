// Initialize knowledge loader
const knowledgeLoader = new RocketshipKnowledgeLoader();

// Generate dynamic tool descriptions based on CLI introspection data
function generateToolDescriptions(knowledgeLoader: RocketshipKnowledgeLoader) {
  const cliData = knowledgeLoader.getCLIData();
  const schema = knowledgeLoader.getSchema();
  const availablePlugins = schema?.properties?.tests?.items?.properties?.steps?.items?.properties?.plugin?.enum || [];

  // Extract dynamic information
  const filePattern = cliData?.usage?.file_structure?.pattern || 'rocketship.yaml';
  const varExamples = cliData?.usage?.syntax_patterns?.variables || {};
  const commonCommands = cliData?.usage?.common_patterns || [];
  
  // Build variable syntax examples from extracted data
  let variableSyntax = '';
  if (varExamples.config && varExamples.config.length > 0) {
    variableSyntax += `💡 Config variables: ${varExamples.config.slice(0, 2).join(', ')}\n`;
  }
  if (varExamples.environment && varExamples.environment.length > 0) {
    variableSyntax += `💡 Environment variables: ${varExamples.environment.slice(0, 2).join(', ')}\n`;
  }
  if (varExamples.runtime && varExamples.runtime.length > 0) {
    variableSyntax += `💡 Runtime variables: ${varExamples.runtime.slice(0, 2).join(', ')}\n`;
  }

  // Build CLI command examples from extracted data
  let cliExamples = '';
  const runCommands = commonCommands.filter((c: any) => c.command.includes('run')).slice(0, 2);
  if (runCommands.length > 0) {
    cliExamples = `💡 Example commands: ${runCommands.map((c: any) => c.command).join(', ')}`;
  }

  // Build plugin recommendations from available plugins
  let pluginRecommendations = '';
  if (availablePlugins.includes('browser')) {
    pluginRecommendations += '💡 For frontend projects: browser plugin available for user journey testing\n';
  }
  if (availablePlugins.includes('http')) {
    pluginRecommendations += '💡 For API projects: http plugin available for endpoint testing\n';
  }

  return {
    get_rocketship_examples: `Provides real examples from the current codebase for specific features or use cases.

💡 YOU (the coding agent) create the test files based on these examples.
${pluginRecommendations}💡 File pattern: ${filePattern}
${variableSyntax}`,

    suggest_test_structure: `Suggests proper file structure and test organization based on current project configuration.

💡 YOU (the coding agent) create the directory structure and files.
${pluginRecommendations}💡 Available plugins: ${availablePlugins.join(', ')}`,

    get_schema_info: `Provides current schema information for validation and proper syntax.

💡 Use this to ensure your YAML follows the correct schema.
💡 Available plugins: ${availablePlugins.join(', ')}
💡 Schema validation ensures compatibility with current version.`,

    get_cli_guidance: `Provides current CLI usage patterns and commands from introspection.

💡 YOU (the coding agent) will run these commands to execute tests.
${cliExamples}
💡 All commands are extracted from current CLI version.`,

    analyze_codebase_for_testing: `Analyzes a codebase to suggest meaningful test scenarios based on available plugins.

💡 Focus on customer-facing flows and critical business logic.
${pluginRecommendations}💡 Suggestions are based on available plugins: ${availablePlugins.join(', ')}
💡 TIP: Include relevant keywords for better flow suggestions`,
  };
}