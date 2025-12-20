package main

import (
	"bufio"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"arguseek/internal/version"
	"gopkg.in/yaml.v3"
)

type Config struct {
	ProjectID      string `yaml:"project_id"`
	Region         string `yaml:"region"`
	ServiceName    string `yaml:"service_name"`
	ServiceAccount string `yaml:"service_account"`
	GoogleCSEID    string `yaml:"google_cse_id"`

	Runtime struct {
		Memory       string `yaml:"memory"`
		CPU          int    `yaml:"cpu"`
		Timeout      int    `yaml:"timeout"`
		Concurrency  int    `yaml:"concurrency"`
		MinInstances int    `yaml:"min_instances"`
		MaxInstances int    `yaml:"max_instances"`
	} `yaml:"runtime"`
}

func main() {
	if len(os.Args) < 2 {
		printUsage()
		os.Exit(1)
	}

	cmd := os.Args[1]

	switch cmd {
	case "dev", "prod":
		dryRun := len(os.Args) > 2 && os.Args[2] == "--dry-run"
		if err := deploy(cmd, dryRun); err != nil {
			log.Fatal(err)
		}
	case "rollback":
		if len(os.Args) < 3 {
			fmt.Println("Error: rollback requires environment argument")
			fmt.Println("Usage: deploy rollback [dev|prod] [-N]")
			os.Exit(1)
		}
		env := os.Args[2]
		if env != "dev" && env != "prod" {
			log.Fatal("Environment must be 'dev' or 'prod'")
		}

		var revision string
		for i := 3; i < len(os.Args); i++ {
			if !strings.HasPrefix(os.Args[i], "--") {
				revision = os.Args[i]
			}
		}

		if err := rollback(env, revision); err != nil {
			log.Fatal(err)
		}
	default:
		fmt.Printf("Unknown command: %s\n\n", cmd)
		printUsage()
		os.Exit(1)
	}
}

func printUsage() {
	fmt.Println("ArguSeek Unified Deploy Tool")
	fmt.Println()
	fmt.Println("Usage:")
	fmt.Println("  deploy [dev|prod] [--dry-run]  # Deploy to environment")
	fmt.Println("  deploy rollback [dev|prod] [-N] # Rollback to previous version")
	fmt.Println()
	fmt.Println("Examples:")
	fmt.Println("  deploy dev                  # Deploy to development")
	fmt.Println("  deploy prod                 # Deploy to production")
	fmt.Println("  deploy prod --dry-run       # Preview production deployment")
	fmt.Println("  deploy rollback prod        # Interactive rollback")
	fmt.Println("  deploy rollback prod -1     # Rollback to previous version")
	fmt.Println("  deploy rollback prod -2     # Rollback 2 versions back")
}

func deploy(env string, dryRun bool) error {
	// Load configuration
	config, err := loadConfig(env)
	if err != nil {
		return fmt.Errorf("failed to load config: %w", err)
	}

	// Load environment variables
	if err := loadEnvVars(env); err != nil {
		return fmt.Errorf("failed to load environment variables: %w", err)
	}

	fmt.Printf("🚀 Deploying ArguSeek to %s environment\n", env)
	fmt.Printf("   Project: %s\n", config.ProjectID)
	fmt.Printf("   Service: %s\n", config.ServiceName)
	fmt.Printf("   Region: %s\n", config.Region)
	fmt.Println()

	if dryRun {
		fmt.Println("DRY RUN MODE - No changes will be made")
		fmt.Println()
		return printDeploymentPlan(config)
	}

	// Pre-deployment setup
	fmt.Println("📋 Running pre-deployment setup...")
	if err := runPreDeploymentSetup(env); err != nil {
		return fmt.Errorf("pre-deployment setup failed: %w", err)
	}

	// Build and push Docker image
	fmt.Println("🏗️  Building Docker image...")
	if err := buildAndPush(config); err != nil {
		return fmt.Errorf("failed to build and push image: %w", err)
	}

	// Deploy to Cloud Run
	fmt.Println("☁️  Deploying to Cloud Run...")
	if err := deployToCloudRun(config); err != nil {
		return fmt.Errorf("failed to deploy to Cloud Run: %w", err)
	}

	// Enable public access
	fmt.Println("🌐 Enabling public access...")
	if err := enablePublicAccess(config); err != nil {
		return fmt.Errorf("failed to enable public access: %w", err)
	}

	// Verify traffic routing
	fmt.Println("🚦 Verifying traffic routing...")
	if err := verifyTrafficRouting(config); err != nil {
		return fmt.Errorf("traffic routing verification failed: %w", err)
	}

	// Tag the deployment
	fmt.Println("🏷️  Tagging deployment revision...")
	if err := tagDeployment(config, env); err != nil {
		// Don't fail deployment if tagging fails
		fmt.Printf("   Warning: Failed to tag revision: %v\n", err)
	}

	// Post-deployment validation
	fmt.Println("✅ Running post-deployment validation...")
	if err := runValidation(env); err != nil {
		return fmt.Errorf("post-deployment validation failed: %w", err)
	}

	fmt.Println()
	fmt.Printf("✨ Deployment to %s completed successfully!\n", env)
	fmt.Printf("   Service URL: https://%s-%s.a.run.app\n", config.ServiceName, config.Region)
	fmt.Println()
	fmt.Println("Next steps:")
	fmt.Println("  1. Run the QA harness: go run tools/qa-harness/main.go " + env)
	fmt.Println("  2. Monitor logs: gcloud logging read --project=" + config.ProjectID)

	return nil
}

func loadConfig(env string) (*Config, error) {
	configPath := filepath.Join("config", env+".yaml")
	data, err := os.ReadFile(configPath)
	if err != nil {
		return nil, err
	}

	var config Config
	if err := yaml.Unmarshal(data, &config); err != nil {
		return nil, err
	}

	return &config, nil
}

func loadEnvVars(env string) error {
	envFile := filepath.Join("config", ".env."+env)
	if _, err := os.Stat(envFile); os.IsNotExist(err) {
		// Check for environment variables
		if os.Getenv("GOOGLE_API_KEY") == "" {
			return fmt.Errorf("GOOGLE_API_KEY not set. Either create %s or set environment variables", envFile)
		}
		return nil
	}

	// Load from .env file
	data, err := os.ReadFile(envFile)
	if err != nil {
		return err
	}

	for _, line := range strings.Split(string(data), "\n") {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}

		parts := strings.SplitN(line, "=", 2)
		if len(parts) == 2 {
			key := strings.TrimSpace(parts[0])
			value := strings.TrimSpace(parts[1])
			if err := os.Setenv(key, value); err != nil {
				return fmt.Errorf("failed to set env %s: %w", key, err)
			}
		}
	}

	return nil
}

func printDeploymentPlan(config *Config) error {
	fmt.Println("Deployment Plan:")
	fmt.Println("----------------")
	fmt.Println()
	fmt.Println("1. Pre-deployment setup")
	fmt.Println("   - Ensure service account exists")
	fmt.Println("   - Configure IAM roles")
	fmt.Println()
	fmt.Println("2. Build and push Docker image")
	fmt.Printf("   - Image: gcr.io/%s/%s:latest\n", config.ProjectID, config.ServiceName)
	fmt.Println()
	fmt.Println("3. Deploy to Cloud Run")
	fmt.Printf("   - Memory: %s\n", config.Runtime.Memory)
	fmt.Printf("   - CPU: %d\n", config.Runtime.CPU)
	fmt.Printf("   - Instances: %d-%d\n", config.Runtime.MinInstances, config.Runtime.MaxInstances)
	fmt.Println()
	fmt.Println("4. Enable public access")
	fmt.Println("   - Allow unauthenticated invocations")

	return nil
}

func runPreDeploymentSetup(env string) error {
	config, err := loadConfig(env)
	if err != nil {
		return fmt.Errorf("failed to load config: %w", err)
	}

	fmt.Println("📋 Checking prerequisites...")

	// Check gcloud command
	if _, err := exec.LookPath("gcloud"); err != nil {
		return fmt.Errorf("gcloud CLI is not installed")
	}

	// Check bq command
	if _, err := exec.LookPath("bq"); err != nil {
		return fmt.Errorf("bq CLI is not installed")
	}

	// Verify gcloud authentication
	cmd := exec.Command("gcloud", "auth", "list", "--filter=status:ACTIVE", "--format=value(account)")
	output, err := cmd.Output()
	if err != nil || len(output) == 0 {
		return fmt.Errorf("no active gcloud authentication found. Run 'gcloud auth login' first")
	}

	// Set project
	cmd = exec.Command("gcloud", "config", "set", "project", config.ProjectID)
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("failed to set project to %s: %w", config.ProjectID, err)
	}

	// Check if service account exists, create if not
	fmt.Printf("   Checking service account %s...\n", config.ServiceAccount)
	cmd = exec.Command("gcloud", "iam", "service-accounts", "describe", config.ServiceAccount,
		"--project", config.ProjectID)
	if err := cmd.Run(); err != nil {
		// Create service account
		fmt.Printf("   Creating service account...\n")
		cmd = exec.Command("gcloud", "iam", "service-accounts", "create", config.ServiceName,
			"--display-name", fmt.Sprintf("ArguSeek %s Service Account", env),
			"--project", config.ProjectID)
		if err := cmd.Run(); err != nil {
			return fmt.Errorf("failed to create service account: %w", err)
		}
	}

	// Grant IAM roles
	fmt.Println("   Checking and granting IAM roles...")
	roles := []string{
		"roles/logging.logWriter",
		"roles/run.viewer",
	}

	for _, role := range roles {
		// Check if role is already granted
		cmd = exec.Command("gcloud", "projects", "get-iam-policy", config.ProjectID,
			"--flatten=bindings[].members",
			"--format=table(bindings.role)",
			"--filter", fmt.Sprintf("bindings.members:serviceAccount:%s AND bindings.role:%s", config.ServiceAccount, role))

		output, _ := cmd.Output()
		if !strings.Contains(string(output), role) {
			// Grant role
			cmd = exec.Command("gcloud", "projects", "add-iam-policy-binding", config.ProjectID,
				"--member", fmt.Sprintf("serviceAccount:%s", config.ServiceAccount),
				"--role", role,
				"--quiet")
			if err := cmd.Run(); err != nil {
				return fmt.Errorf("failed to grant %s: %w", role, err)
			}
			fmt.Printf("     ✓ Granted %s\n", role)
		} else {
			fmt.Printf("     ✓ %s already granted\n", role)
		}
	}

	// Enable required APIs
	fmt.Println("   Enabling required APIs...")
	apis := []string{
		"run.googleapis.com",
		"logging.googleapis.com",
	}

	for _, api := range apis {
		cmd = exec.Command("gcloud", "services", "enable", api, "--project", config.ProjectID)
		if err := cmd.Run(); err != nil {
			// API might already be enabled, continue
			fmt.Printf("     ⚠ %s might already be enabled\n", api)
		} else {
			fmt.Printf("     ✓ %s enabled\n", api)
		}
	}

	// Validate environment variables
	fmt.Println("   Validating environment variables...")
	requiredVars := []string{"GOOGLE_API_KEY"}
	for _, envVar := range requiredVars {
		if os.Getenv(envVar) == "" {
			return fmt.Errorf("environment variable %s is not set", envVar)
		}
	}

	return nil
}

func buildAndPush(config *Config) error {
	imageName := fmt.Sprintf("gcr.io/%s/%s:latest", config.ProjectID, config.ServiceName)

	// Get version from single source of truth (version package)
	ver := version.Version
	fmt.Printf("Building Docker image with version: %s\n", ver)

	// Build image with version injection
	cmd := exec.Command("docker", "build",
		"--platform", "linux/amd64",
		"--build-arg", fmt.Sprintf("VERSION=%s", ver),
		"-t", imageName,
		".")
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("docker build failed: %w", err)
	}

	// Push image
	cmd = exec.Command("docker", "push", imageName)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("docker push failed: %w", err)
	}

	return nil
}

func deployToCloudRun(config *Config) error {
	imageName := fmt.Sprintf("gcr.io/%s/%s:latest", config.ProjectID, config.ServiceName)

	// Use Vertex AI in Cloud Run deployments (better quotas, IAM auth)
	// The project ID enables the Vertex AI backend automatically
	envVars := fmt.Sprintf(
		"GOOGLE_API_KEY=%s,GOOGLE_CSE_ID=%s,GOOGLE_CLOUD_PROJECT=%s,GOOGLE_CLOUD_LOCATION=%s",
		os.Getenv("GOOGLE_API_KEY"),
		config.GoogleCSEID,
		config.ProjectID,
		config.Region,
	)

	args := []string{
		"run", "deploy", config.ServiceName,
		"--image", imageName,
		"--region", config.Region,
		"--platform", "managed",
		"--set-env-vars", envVars,
		"--memory", config.Runtime.Memory,
		"--cpu", fmt.Sprintf("%d", config.Runtime.CPU),
		"--timeout", fmt.Sprintf("%d", config.Runtime.Timeout),
		"--concurrency", fmt.Sprintf("%d", config.Runtime.Concurrency),
		"--max-instances", fmt.Sprintf("%d", config.Runtime.MaxInstances),
		"--min-instances", fmt.Sprintf("%d", config.Runtime.MinInstances),
		"--service-account", config.ServiceAccount,
		"--project", config.ProjectID,
		"--quiet",
	}

	cmd := exec.Command("gcloud", args...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}

func enablePublicAccess(config *Config) error {
	args := []string{
		"run", "services", "add-iam-policy-binding", config.ServiceName,
		"--region", config.Region,
		"--project", config.ProjectID,
		"--member", "allUsers",
		"--role", "roles/run.invoker",
		"--quiet",
	}

	cmd := exec.Command("gcloud", args...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}

func verifyTrafficRouting(config *Config) error {
	fmt.Println("   Verifying traffic routing...")

	// Exponential backoff configuration
	initialDelay := 1 * time.Second
	maxDelay := 30 * time.Second
	maxAttempts := 10
	backoffMultiplier := 2.0

	delay := initialDelay

	for attempt := 1; attempt <= maxAttempts; attempt++ {
		// Get latest revision and current traffic allocation
		cmd := exec.Command("gcloud", "run", "services", "describe", config.ServiceName,
			"--region", config.Region,
			"--project", config.ProjectID,
			"--format", "json")

		output, err := cmd.Output()
		if err != nil {
			return fmt.Errorf("failed to get service description: %w", err)
		}

		var serviceData map[string]interface{}
		if err := json.Unmarshal(output, &serviceData); err != nil {
			return fmt.Errorf("failed to parse service data: %w", err)
		}

		// Get latest ready revision
		status := serviceData["status"].(map[string]interface{})
		latestRevision, ok := status["latestReadyRevisionName"].(string)
		if !ok {
			if attempt == maxAttempts {
				return fmt.Errorf("could not find latest ready revision after %d attempts", maxAttempts)
			}
			time.Sleep(delay)
			delay = time.Duration(float64(delay) * backoffMultiplier)
			if delay > maxDelay {
				delay = maxDelay
			}
			continue
		}

		// Check traffic allocation in status (not spec)
		statusTraffic, ok := status["traffic"].([]interface{})
		if ok && len(statusTraffic) > 0 {
			// Check if traffic is routed correctly
			for _, t := range statusTraffic {
				trafficItem := t.(map[string]interface{})
				if revisionName, hasRevision := trafficItem["revisionName"].(string); hasRevision {
					if revisionName == latestRevision {
						if percent, hasPercent := trafficItem["percent"].(float64); hasPercent {
							if percent == 100 {
								fmt.Printf("     ✓ Traffic correctly routed to latest revision: %s (100%%)\n", latestRevision)
								return nil
							}
						}
					}
				}
			}
		}

		// If this is the last attempt, provide detailed error
		if attempt == maxAttempts {
			var actualPercent float64
			for _, t := range statusTraffic {
				trafficItem := t.(map[string]interface{})
				if revisionName, hasRevision := trafficItem["revisionName"].(string); hasRevision {
					if revisionName == latestRevision {
						if percent, hasPercent := trafficItem["percent"].(float64); hasPercent {
							actualPercent = percent
						}
					}
				}
			}
			return fmt.Errorf("traffic not fully routed to latest revision %s (only %.0f%%) after %d attempts",
				latestRevision, actualPercent, maxAttempts)
		}

		// Wait before next attempt
		fmt.Printf("     Waiting for traffic migration (attempt %d/%d)...\n", attempt, maxAttempts)
		time.Sleep(delay)

		// Exponential increase with cap
		delay = time.Duration(float64(delay) * backoffMultiplier)
		if delay > maxDelay {
			delay = maxDelay
		}
	}

	return fmt.Errorf("traffic routing verification timed out after %d attempts", maxAttempts)
}

func runValidation(env string) error {
	config, err := loadConfig(env)
	if err != nil {
		return fmt.Errorf("failed to load config: %w", err)
	}

	fmt.Println("✅ Running post-deployment validation...")

	// Check if service exists and is ready
	fmt.Println("   Checking Cloud Run service status...")
	cmd := exec.Command("gcloud", "run", "services", "describe", config.ServiceName,
		"--region", config.Region,
		"--project", config.ProjectID,
		"--format", "value(status.conditions[0].status)")

	output, err := cmd.Output()
	if err != nil {
		return fmt.Errorf("service %s not found or not accessible", config.ServiceName)
	}

	if strings.TrimSpace(string(output)) != "True" {
		return fmt.Errorf("service is not ready. Status: %s", string(output))
	}
	fmt.Println("     ✓ Service is ready and serving traffic")

	// Get service URL
	cmd = exec.Command("gcloud", "run", "services", "describe", config.ServiceName,
		"--region", config.Region,
		"--project", config.ProjectID,
		"--format", "value(status.url)")

	urlOutput, err := cmd.Output()
	if err != nil {
		return fmt.Errorf("failed to get service URL: %w", err)
	}
	serviceURL := strings.TrimSpace(string(urlOutput))

	// Validate environment variables
	fmt.Println("   Validating environment variables...")
	cmd = exec.Command("gcloud", "run", "services", "describe", config.ServiceName,
		"--region", config.Region,
		"--project", config.ProjectID,
		"--format", "json")

	output, err = cmd.Output()
	if err != nil {
		return fmt.Errorf("failed to get service configuration: %w", err)
	}

	// Parse JSON to check environment variables
	var serviceSpec map[string]interface{}
	if err := json.Unmarshal(output, &serviceSpec); err != nil {
		return fmt.Errorf("failed to parse service configuration: %w", err)
	}

	// Navigate through the nested structure to get env vars
	spec, _ := serviceSpec["spec"].(map[string]interface{})
	template, _ := spec["template"].(map[string]interface{})
	templateSpec, _ := template["spec"].(map[string]interface{})
	containers, _ := templateSpec["containers"].([]interface{})

	if len(containers) == 0 {
		return fmt.Errorf("no containers found in service")
	}

	container := containers[0].(map[string]interface{})
	envVars, _ := container["env"].([]interface{})

	// Check required environment variables
	// GOOGLE_CLOUD_PROJECT enables Vertex AI backend (better quotas)
	requiredVars := []string{
		"GOOGLE_API_KEY",
		"GOOGLE_CSE_ID",
		"GOOGLE_CLOUD_PROJECT",
	}

	envMap := make(map[string]string)
	for _, envVar := range envVars {
		env := envVar.(map[string]interface{})
		name := env["name"].(string)
		value, _ := env["value"].(string)
		envMap[name] = value
	}

	missingVars := []string{}
	for _, required := range requiredVars {
		if _, exists := envMap[required]; !exists {
			missingVars = append(missingVars, required)
		}
	}

	if len(missingVars) > 0 {
		return fmt.Errorf("missing required environment variables: %v", missingVars)
	}

	fmt.Println("     ✓ All environment variables correctly set")

	// Test service health
	fmt.Println("   Testing service health...")
	healthURL := serviceURL + "/health"

	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Get(healthURL)
	if err != nil {
		return fmt.Errorf("health check request failed: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("health check failed. HTTP status: %d", resp.StatusCode)
	}
	fmt.Printf("     ✓ Health check passed (HTTP %d)\n", resp.StatusCode)

	// Test MCP endpoint
	fmt.Println("   Testing MCP endpoint...")
	payload := map[string]interface{}{
		"jsonrpc": "2.0",
		"id":      1,
		"method":  "tools/list",
		"params":  map[string]interface{}{},
	}

	payloadBytes, _ := json.Marshal(payload)
	req, err := http.NewRequest("POST", serviceURL+"/mcp", strings.NewReader(string(payloadBytes)))
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")

	resp, err = client.Do(req)
	if err != nil {
		return fmt.Errorf("MCP endpoint test request failed: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("MCP endpoint test failed. HTTP status: %d", resp.StatusCode)
	}
	fmt.Println("     ✓ MCP endpoint test passed")

	// Check IAM permissions
	fmt.Println("   Checking IAM permissions...")
	serviceAccount := envMap["SERVICE_ACCOUNT"]
	if serviceAccount == "" {
		// Get from service configuration
		serviceAccount, _ = templateSpec["serviceAccountName"].(string)
		if serviceAccount == "" {
			serviceAccount = config.ServiceAccount
		}
	}

	roles := []string{
		"roles/logging.logWriter",
		"roles/run.viewer",
	}

	for _, role := range roles {
		cmd = exec.Command("gcloud", "projects", "get-iam-policy", config.ProjectID,
			"--flatten=bindings[].members",
			"--format=table(bindings.role)",
			"--filter", fmt.Sprintf("bindings.members:serviceAccount:%s AND bindings.role:%s", serviceAccount, role))

		output, _ := cmd.Output()
		if !strings.Contains(string(output), role) {
			return fmt.Errorf("missing IAM role: %s for service account: %s", role, serviceAccount)
		}
	}
	fmt.Println("     ✓ All IAM roles granted")

	fmt.Println()
	fmt.Printf("✨ Post-deployment validation completed successfully!\n")
	fmt.Printf("   Environment: %s\n", env)
	fmt.Printf("   Service: %s\n", config.ServiceName)
	fmt.Printf("   URL: %s\n", serviceURL)

	return nil
}

func tagDeployment(config *Config, env string) error {
	timestamp := time.Now().Format("20060102-150405")

	// Generate shorter tag to avoid Google Cloud Run 46-character limit
	// Format: env-timestamp (removing redundant service name)
	tag := fmt.Sprintf("%s-%s", env, timestamp)

	// Validate combined length (tag + service name must be <= 46 chars)
	combinedLength := len(tag) + len(config.ServiceName)
	if combinedLength > 46 {
		// Use shorter timestamp format if needed
		shortTimestamp := time.Now().Format("0102-1504")
		tag = fmt.Sprintf("%s-%s", env, shortTimestamp)
		combinedLength = len(tag) + len(config.ServiceName)

		if combinedLength > 46 {
			// Skip tagging if still too long
			fmt.Printf("   Warning: Skipping tag due to length constraints (would be %d chars, max 46)\n", combinedLength)
			return nil
		}
	}

	// Get the current revision name
	cmd := exec.Command("gcloud", "run", "services", "describe", config.ServiceName,
		"--region", config.Region,
		"--project", config.ProjectID,
		"--format", "value(status.latestReadyRevisionName)")

	output, err := cmd.Output()
	if err != nil {
		return fmt.Errorf("failed to get current revision: %w", err)
	}

	revisionName := strings.TrimSpace(string(output))
	if revisionName == "" {
		return fmt.Errorf("no revision found")
	}

	// Tag the revision (using set-tags which doesn't affect traffic)
	cmd = exec.Command("gcloud", "run", "services", "update-traffic", config.ServiceName,
		"--set-tags", tag+"="+revisionName,
		"--region", config.Region,
		"--project", config.ProjectID,
		"--quiet")

	if output, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("failed to tag revision: %w\n%s", err, output)
	}

	fmt.Printf("   Tagged revision %s as %s\n", revisionName, tag)
	return nil
}

func rollback(env string, revisionSpec string) error {
	// Load configuration
	config, err := loadConfig(env)
	if err != nil {
		return fmt.Errorf("failed to load config: %w", err)
	}

	fmt.Printf("🔄 Rolling back ArguSeek %s environment\n", env)
	fmt.Printf("   Project: %s\n", config.ProjectID)
	fmt.Printf("   Service: %s\n", config.ServiceName)
	fmt.Println()

	// Get list of revisions
	revisions, err := getRevisions(config)
	if err != nil {
		return fmt.Errorf("failed to get revisions: %w", err)
	}

	if len(revisions) == 0 {
		return fmt.Errorf("no revisions found")
	}

	var targetRevision string

	if revisionSpec == "" {
		// Interactive mode
		targetRevision, err = selectRevisionInteractive(revisions)
		if err != nil {
			return err
		}
	} else if strings.HasPrefix(revisionSpec, "-") {
		// Relative revision (e.g., -1, -2)
		offset, err := strconv.Atoi(revisionSpec[1:])
		if err != nil {
			return fmt.Errorf("invalid revision offset: %s", revisionSpec)
		}

		if offset >= len(revisions) {
			return fmt.Errorf("not enough revisions (requested -%d, but only %d available)", offset, len(revisions))
		}

		targetRevision = revisions[offset].Name
		fmt.Printf("Selected revision: %s (%s)\n", targetRevision, revisions[offset].Tag)
	} else {
		return fmt.Errorf("invalid revision specifier: %s", revisionSpec)
	}

	// Confirm rollback
	fmt.Printf("\n⚠️  About to rollback to revision: %s\n", targetRevision)
	fmt.Print("Continue? [y/N]: ")

	reader := bufio.NewReader(os.Stdin)
	response, _ := reader.ReadString('\n')
	response = strings.TrimSpace(strings.ToLower(response))

	if response != "y" && response != "yes" {
		fmt.Println("Rollback cancelled")
		return nil
	}

	// Perform rollback
	fmt.Println("\n🚀 Rolling back...")
	if err := performRollback(config, targetRevision); err != nil {
		return fmt.Errorf("rollback failed: %w", err)
	}

	// Run validation
	fmt.Println("\n✅ Running post-rollback validation...")
	if err := runValidation(env); err != nil {
		return fmt.Errorf("post-rollback validation failed: %w", err)
	}

	fmt.Println("\n✨ Rollback completed successfully!")
	fmt.Printf("   Service URL: https://%s-%s.a.run.app\n", config.ServiceName, config.Region)

	return nil
}

type Revision struct {
	Name      string
	Tag       string
	Timestamp string
	Current   bool
}

func getRevisions(config *Config) ([]Revision, error) {
	// Get all revisions with their metadata
	cmd := exec.Command("gcloud", "run", "revisions", "list",
		"--service", config.ServiceName,
		"--region", config.Region,
		"--project", config.ProjectID,
		"--format", "json")

	output, err := cmd.Output()
	if err != nil {
		return nil, err
	}

	var revisionData []map[string]interface{}
	if err := json.Unmarshal(output, &revisionData); err != nil {
		return nil, err
	}

	// Get current revision
	cmd = exec.Command("gcloud", "run", "services", "describe", config.ServiceName,
		"--region", config.Region,
		"--project", config.ProjectID,
		"--format", "value(status.latestReadyRevisionName)")

	currentOutput, _ := cmd.Output()
	currentRevision := strings.TrimSpace(string(currentOutput))

	// Get tagged revisions
	cmd = exec.Command("gcloud", "run", "services", "describe", config.ServiceName,
		"--region", config.Region,
		"--project", config.ProjectID,
		"--format", "json")

	serviceOutput, err := cmd.Output()
	if err != nil {
		return nil, err
	}

	var serviceData map[string]interface{}
	if err := json.Unmarshal(serviceOutput, &serviceData); err != nil {
		return nil, err
	}

	// Extract tags from traffic allocation
	tags := make(map[string]string)
	if spec, ok := serviceData["spec"].(map[string]interface{}); ok {
		if traffic, ok := spec["traffic"].([]interface{}); ok {
			for _, t := range traffic {
				if trafficItem, ok := t.(map[string]interface{}); ok {
					if tag, hasTag := trafficItem["tag"].(string); hasTag {
						if revision, hasRevision := trafficItem["revisionName"].(string); hasRevision {
							tags[revision] = tag
						}
					}
				}
			}
		}
	}

	var revisions []Revision
	for _, data := range revisionData {
		name := data["metadata"].(map[string]interface{})["name"].(string)

		// Parse creation timestamp
		createdAt := data["metadata"].(map[string]interface{})["creationTimestamp"].(string)
		timestamp, _ := time.Parse(time.RFC3339, createdAt)

		revision := Revision{
			Name:      name,
			Tag:       tags[name],
			Timestamp: timestamp.Format("2006-01-02 15:04:05"),
			Current:   name == currentRevision,
		}

		revisions = append(revisions, revision)
	}

	return revisions, nil
}

func selectRevisionInteractive(revisions []Revision) (string, error) {
	fmt.Println("Recent deployments:")
	fmt.Println()

	for i, rev := range revisions {
		if i >= 10 {
			break // Show only last 10
		}

		status := ""
		if rev.Current {
			status = " (current)"
		}

		tag := ""
		if rev.Tag != "" {
			tag = fmt.Sprintf(" [%s]", rev.Tag)
		}

		fmt.Printf("%2d. %s - %s%s%s\n", i+1, rev.Timestamp, rev.Name, tag, status)
	}

	fmt.Print("\nSelect version to rollback to [1-10]: ")

	reader := bufio.NewReader(os.Stdin)
	input, err := reader.ReadString('\n')
	if err != nil {
		return "", err
	}

	choice, err := strconv.Atoi(strings.TrimSpace(input))
	if err != nil || choice < 1 || choice > len(revisions) || choice > 10 {
		return "", fmt.Errorf("invalid selection")
	}

	return revisions[choice-1].Name, nil
}

func performRollback(config *Config, targetRevision string) error {
	// Update traffic to 100% on target revision
	cmd := exec.Command("gcloud", "run", "services", "update-traffic", config.ServiceName,
		"--to-revisions", targetRevision+"=100",
		"--region", config.Region,
		"--project", config.ProjectID,
		"--quiet")

	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	if err := cmd.Run(); err != nil {
		return err
	}

	fmt.Printf("   Traffic rolled back to revision: %s\n", targetRevision)
	return nil
}
