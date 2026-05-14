/**
 * TypeScript definitions for Mooncake configuration
 * 
 * Auto-generated from action metadata.
 * Do not edit manually - regenerate with: mooncake schema generate --format typescript
 */

/**
 * Capture file changes with enhanced metadata for LLM agents
 * @category system
 */
export interface ArtifactCaptureAction {
  /**
   * Capture full file content before/after
   */
  capture_content?: boolean;
  /**
   * Embed full plan in artifact for LLM context
   */
  embed_plan?: boolean;
  /**
   * Output format
   * 
   * @values json | markdown | both
   */
  format?: "json" | "markdown" | "both";
  /**
   * Include SHA256 checksums
   */
  include_checksums?: boolean;
  /**
   * Maximum diff size in bytes per file
   */
  max_diff_size?: number;
  /**
   * Don't embed plan if exceeds this many steps
   */
  max_plan_steps?: number;
  /**
   * Name of the artifact (used for output directory)
   */
  name: string;
  /**
   * Base directory for artifacts (default: './artifacts')
   */
  output_dir?: string;
  /**
   * Steps to execute while capturing changes
   */
  steps: StepAction[];
}

/**
 * Validate artifacts against constraints (change budgets)
 * @category system
 */
export interface ArtifactValidateAction {
  /**
   * Glob patterns for allowed file paths
   */
  allowed_paths?: string[];
  /**
   * Path to artifact metadata JSON file
   */
  artifact_file: string;
  /**
   * Glob patterns for forbidden file paths
   */
  forbidden_paths?: string[];
  /**
   * Maximum file size in bytes after changes
   */
  max_file_size?: number;
  /**
   * Maximum number of files allowed to change
   */
  max_files?: number;
  /**
   * Maximum total lines changed
   */
  max_lines_changed?: number;
  /**
   * Require test file changes when code files change
   */
  require_tests?: boolean;
}

/**
 * Verify conditions without changing system state
 * @category system
 */
export interface AssertAction {
  command?: {
    cmd: string;
    exit_code: number;
  };
  file?: {
    contains: string;
    content: string;
    exists: boolean;
    group: string;
    mode: string;
    owner: string;
    path: string;
  };
  file_sha256?: {
    checksum: string;
    path: string;
  };
  git_clean?: {
    allow_untracked: boolean;
  };
  git_diff?: {
    cached: boolean;
    expected_diff: string;
    files: string;
  };
  http?: {
    body: string;
    body_equals: string;
    contains: string;
    headers: Record<string, any>;
    jsonpath: string;
    jsonpath_value: any;
    method: string;
    status: number;
    timeout: string;
    url: string;
  };
}

/**
 * Execute commands directly without shell interpolation
 * @category command
 */
export interface CmdAction {
  argv: string[];
  capture?: boolean;
  stdin?: string;
}

/**
 * Execute commands directly without shell interpolation
 * @category command
 */
export interface CmdActionAction {
  argv: string[];
  capture?: boolean;
  stdin?: string;
}

/**
 * Manage container lifecycle (running/stopped/absent) via podman or docker
 * 
 * @platforms linux, darwin, windows
 * @category system
 */
export interface ContainerAction {
  command?: string[];
  env?: Record<string, any>;
  extra?: string[];
  image?: string;
  name: string;
  network?: string;
  ports?: string[];
  recreate?: boolean;
  restart?: string;
  runtime?: string;
  state?: string;
  volumes?: string[];
}

/**
 * Manage container images (pull/remove) via podman or docker
 * 
 * @platforms linux, darwin, windows
 * @category system
 */
export interface ContainerImageAction {
  force_pull?: boolean;
  name: string;
  runtime?: string;
  state?: string;
}

/**
 * Copy files with checksum verification and atomic writes
 * @category file
 */
export interface FileCopyAction {
  backup?: boolean;
  checksum?: string;
  dest: string;
  force?: boolean;
  group?: string;
  mode?: string;
  owner?: string;
  src: string;
}

/**
 * Download files from URLs with checksum verification
 * @category network
 */
export interface FileDownloadAction {
  backup?: boolean;
  checksum?: string;
  dest: string;
  force?: boolean;
  headers?: Record<string, any>;
  mode?: string;
  retries?: number;
  timeout?: string;
  url: string;
}

/**
 * Render template files and write to destination
 * @category file
 */
export interface FileTemplateAction {
  dest: string;
  mode?: string;
  src: string;
  vars?: Record<string, any>;
}

/**
 * Extract archive files (tar, tar.gz, zip) with path traversal protection
 * @category file
 */
export interface FileUnarchiveAction {
  creates?: string;
  dest: string;
  mode?: string;
  src: string;
  strip_components?: number;
}

/**
 * Manage files, directories, links, and permissions
 * @category file
 */
export interface FileWriteAction {
  backup?: boolean;
  content?: string;
  force?: boolean;
  /**
   * File group (groupname or GID)
   */
  group?: string;
  /**
   * File permissions (e.g., '0644', '0755')
   */
  mode?: string;
  /**
   * File owner (username or UID)
   */
  owner?: string;
  /**
   * File, directory, or symlink path (required)
   */
  path: string;
  recurse?: boolean;
  src?: string;
  /**
   * Desired file state (file/present: file exists, absent: removed,
   * directory: dir exists, link: symlink, hardlink: hard link, touch:
   * update timestamp, perms: change permissions only)
   * 
   * @values file | present | absent | directory | link | hardlink | touch | perms
   */
  state?: "file" | "present" | "absent" | "directory" | "link" | "hardlink" | "touch" | "perms";
}

/**
 * Import steps from another file
 */
export interface ImportAction {
}

/**
 * Display messages to the user
 * @category output
 */
export interface LogAction {
  msg?: string;
}

/**
 * Manage services across platforms (systemd, launchd, Windows)
 * 
 * @platforms linux, darwin, windows
 * @requiresSudo true
 * @category system
 */
export interface OsServiceAction {
  /**
   * Run 'systemctl daemon-reload' after unit file changes (systemd only)
   */
  daemon_reload?: boolean;
  dropin?: {
    content: string;
    name: string;
    src_template: string;
  };
  /**
   * Enable service to start on boot (systemd: enable/disable, launchd:
   * bootstrap/bootout)
   */
  enabled?: boolean;
  /**
   * Service name (systemd: nginx, launchd: com.example.app)
   */
  name: string;
  /**
   * Desired service state
   * 
   * @values started | stopped | restarted | reloaded
   */
  state?: "started" | "stopped" | "restarted" | "reloaded";
  unit?: {
    content: string;
    dest: string;
    mode: string;
    src_template: string;
  };
}

/**
 * Manage system packages (install/remove/update)
 * 
 * @platforms linux, darwin, windows, freebsd
 * @requiresSudo true
 * @category system
 */
export interface PkgAction {
  extra?: string[];
  /**
   * Package manager (auto-detected if empty: apt, dnf, yum, pacman,
   * zypper, apk, brew, port, choco, scoop)
   */
  manager?: string;
  /**
   * Package name (single package)
   */
  name?: string;
  /**
   * Multiple packages to install/remove
   */
  names?: string[];
  /**
   * Package state (present: installed, absent: removed, latest: install or
   * upgrade)
   * 
   * @values present | absent | latest
   */
  state?: "present" | "absent" | "latest";
  /**
   * Update package cache before operation (e.g., apt-get update)
   */
  update_cache?: boolean;
  upgrade?: boolean;
}

/**
 * Manage a third-party package repository (apt; dnf/brew deferred)
 * 
 * @platforms linux
 * @requiresSudo true
 * @category system
 */
export interface PkgRepoAction {
  apt?: {
    architectures: string[];
    components: string[];
    gpg_check: boolean;
    gpg_key_fingerprint: string;
    gpg_key_url: string;
    suites: string[];
    update_cache: boolean;
    uri: string;
  };
  brew?: {
    tap: string;
  };
  dnf?: {
    baseurl: string;
    gpg_check: boolean;
    gpg_key_url: string;
  };
  name: string;
  state?: string;
}

/**
 * Apply multiple patches to multiple files atomically
 * @category file
 */
export interface RepoPatchAction {
  backup?: boolean;
  base_dir?: string;
  dry_run?: boolean;
  output_file?: string;
  patchset?: string;
  patchset_file?: string;
  strict?: boolean;
}

/**
 * Search codebase for patterns and output results in JSON format
 * @category file
 */
export interface RepoSearchAction {
  glob?: string;
  ignore_dirs?: string[];
  max_results?: number;
  output_file?: string;
  path?: string;
  pattern: string;
  regex?: boolean;
}

/**
 * Generate a JSON representation of directory structure
 * @category file
 */
export interface RepoTreeAction {
  exclude_dirs?: string[];
  include_files?: boolean;
  max_depth?: number;
  output_file?: string;
  path?: string;
}

/**
 * Structured configuration with version, global variables, and steps
 */
export interface RunConfigAction {
  /**
   * Configuration steps to execute
   */
  steps: StepAction[];
  /**
   * Global variables available to all steps
   */
  vars?: Record<string, any>;
  /**
   * Configuration schema version (e.g., '1.0')
   */
  version?: string;
}

/**
 * Execute shell commands
 * @category command
 */
export interface ShellAction {
  /**
   * Capture command output (default: true). When false, output is only
   * streamed
   */
  capture?: boolean;
  /**
   * Shell command to execute (required)
   */
  cmd?: string;
  /**
   * Shell interpreter (bash, sh, pwsh, cmd). Default: bash on Unix, pwsh
   * on Windows
   * 
   * @values bash | sh | pwsh | cmd
   */
  interpreter?: "bash" | "sh" | "pwsh" | "cmd";
  /**
   * Input to provide to the command via stdin
   */
  stdin?: string;
}

/**
 * Execute shell commands
 * @category command
 */
export interface ShellActionAction {
  /**
   * Capture command output (default: true). When false, output is only
   * streamed
   */
  capture?: boolean;
  /**
   * Shell command to execute (required)
   */
  cmd?: string;
  /**
   * Shell interpreter (bash, sh, pwsh, cmd). Default: bash on Unix, pwsh
   * on Windows
   * 
   * @values bash | sh | pwsh | cmd
   */
  interpreter?: "bash" | "sh" | "pwsh" | "cmd";
  /**
   * Input to provide to the command via stdin
   */
  stdin?: string;
}

/**
 * Delete text between start and end anchor patterns in files
 * @category file
 */
export interface TextDeleteRangeAction {
  backup?: boolean;
  end_anchor: string;
  inclusive?: boolean;
  path: string;
  regex?: boolean;
  start_anchor: string;
}

/**
 * Insert text before or after anchor patterns in files
 * @category file
 */
export interface TextInsertAction {
  allow_multiple?: boolean;
  anchor: string;
  backup?: boolean;
  content: string;
  path: string;
  position: string;
  regex?: boolean;
}

/**
 * Apply unified diff patches to files
 * @category file
 */
export interface TextPatchAction {
  backup?: boolean;
  context_lines?: number;
  dry_run?: boolean;
  patch?: string;
  patch_file?: string;
  path: string;
  strict?: boolean;
}

/**
 * Replace text in files using literal or regex patterns
 * @category file
 */
export interface TextReplaceAction {
  allow_no_match?: boolean;
  backup?: boolean;
  count?: number;
  flags?: {
    case_insensitive: boolean;
    multiline: boolean;
    regex: boolean;
  };
  path: string;
  pattern: string;
  replace: string;
}

/**
 * Install a developer tool at a pinned version with lockfile-backed reproducibility
 * 
 * @platforms linux, darwin
 * @category system
 */
export interface ToolAction {
  asset?: string;
  backend: string;
  bin?: string;
  checksum?: string;
  env?: Record<string, any>;
  mise_tool?: string;
  name: string;
  repo?: string;
  strip_components?: number;
  tag?: string;
  url?: string;
  version: string;
  write_tool_versions?: boolean;
}

/**
 * Execute a preset by expanding it into steps
 * @category system
 */
export interface UseAction {
  name: string;
  with?: Record<string, any>;
}

/**
 * Define or update variables
 * @category data
 */
export interface VarsAction {
}

/**
 * Load variables from a YAML file
 * @category data
 */
export interface VarsLoadAction {
}

/**
 * Wait for a shell command to exit with the expected code
 * @category command
 */
export interface WaitCommandAction {
  cmd: string;
  expect_exit?: number;
  poll_interval?: string;
  timeout?: string;
}

/**
 * Wait for a file or directory to exist (optionally containing a substring)
 * @category system
 */
export interface WaitFileAction {
  contains?: string;
  path: string;
  poll_interval?: string;
  timeout?: string;
}

/**
 * Wait for an HTTP endpoint to return an accepted status
 * @category network
 */
export interface WaitHttpAction {
  body_contains?: string;
  headers?: Record<string, any>;
  method?: string;
  poll_interval?: string;
  status?: number[];
  timeout?: string;
  url: string;
}

/**
 * Wait for a TCP port to accept connections
 * @category network
 */
export interface WaitPortAction {
  host?: string;
  poll_interval?: string;
  port: number;
  timeout?: string;
}

/**
 * A single configuration step
 * 
 * Each step must contain exactly one action (shell, file, service, etc.)
 * plus optional universal fields (name, when, register, etc.)
 */
export interface Step {
  /**
   * Name of the step (universal)
   */
  name?: string;
  /**
   * Conditional expression for step execution (universal)
   */
  when?: string;
  /**
   * Skip step if this file path exists. Useful for idempotency (universal)
   */
  unless_exists?: string;
  /**
   * Skip step if this command succeeds (exit code 0). Useful for
   * idempotency (universal)
   */
  unless_command?: string;
  /**
   * Run as this user (empty = current user, 'root' = sudo to root,
   * '<name>' = sudo to user). Works with: shell, cmd, file.write,
   * file.template
   */
  as_user?: string;
  /**
   * Tags for filtering step execution (universal)
   */
  tags?: string[];
  /**
   * Variable name to store step execution outputs (universal)
   */
  as?: string;
  /**
   * Directory path for iterating over files (universal)
   */
  for_each_file?: string;
  /**
   * Variable expression for iterating over items (universal)
   */
  for_each?: string;
  /**
   * Environment variables for the step
   */
  env?: Record<string, any>;
  /**
   * Working directory for the step
   */
  cwd?: string;
  /**
   * ⚠️ shell/cmd ONLY: Maximum execution time (e.g., '30s', '5m',
   * '1h')
   */
  timeout?: string;
  /**
   * Retry policy: { attempts: int, delay: string, backoff: string }
   */
  retry?: object;
  /**
   * Continue execution even if this step fails (universal)
   */
  continue_on_error?: boolean;
  /**
   * Expression to override changed result
   */
  changed_when?: string;
  /**
   * Expression to override failure condition
   */
  failed_when?: string;
  /**
   * Path to YAML file with steps to import
   */
  import?: string;

  // Action fields (exactly one must be specified)
  /**
   * Capture file changes with enhanced metadata for LLM agents
   */
  "artifact.capture"?: ArtifactCaptureAction;
  /**
   * Validate artifacts against constraints (change budgets)
   */
  "artifact.validate"?: ArtifactValidateAction;
  /**
   * Verify conditions without changing system state
   */
  assert?: AssertAction;
  /**
   * Execute commands directly without shell interpolation
   */
  cmd?: CmdAction;
  /**
   * Manage container lifecycle (running/stopped/absent) via podman or
   * docker
   */
  container?: ContainerAction;
  /**
   * Manage container images (pull/remove) via podman or docker
   */
  "container.image"?: ContainerImageAction;
  /**
   * Copy files with checksum verification and atomic writes
   */
  "file.copy"?: FileCopyAction;
  /**
   * Download files from URLs with checksum verification
   */
  "file.download"?: FileDownloadAction;
  /**
   * Render template files and write to destination
   */
  "file.template"?: FileTemplateAction;
  /**
   * Extract archive files (tar, tar.gz, zip) with path traversal
   * protection
   */
  "file.unarchive"?: FileUnarchiveAction;
  /**
   * Manage files, directories, links, and permissions
   */
  "file.write"?: FileWriteAction;
  /**
   * Display messages to the user
   */
  log?: LogAction;
  /**
   * Manage services across platforms (systemd, launchd, Windows)
   */
  "os.service"?: OsServiceAction;
  /**
   * Manage system packages (install/remove/update)
   */
  pkg?: PkgAction;
  /**
   * Manage a third-party package repository (apt; dnf/brew deferred)
   */
  "pkg.repo"?: PkgRepoAction;
  /**
   * Apply multiple patches to multiple files atomically
   */
  "repo.patch"?: RepoPatchAction;
  /**
   * Search codebase for patterns and output results in JSON format
   */
  "repo.search"?: RepoSearchAction;
  /**
   * Generate a JSON representation of directory structure
   */
  "repo.tree"?: RepoTreeAction;
  /**
   * Execute shell commands
   */
  shell?: string | ShellAction;
  /**
   * Delete text between start and end anchor patterns in files
   */
  "text.delete_range"?: TextDeleteRangeAction;
  /**
   * Insert text before or after anchor patterns in files
   */
  "text.insert"?: TextInsertAction;
  /**
   * Apply unified diff patches to files
   */
  "text.patch"?: TextPatchAction;
  /**
   * Replace text in files using literal or regex patterns
   */
  "text.replace"?: TextReplaceAction;
  /**
   * Install a developer tool at a pinned version with lockfile-backed
   * reproducibility
   */
  tool?: ToolAction;
  /**
   * Execute a preset by expanding it into steps
   */
  use?: string | UseAction;
  /**
   * Set variables for use in subsequent steps
   */
  vars?: VarsAction;
  /**
   * Load variables from YAML files
   */
  "vars.load"?: VarsLoadAction;
  /**
   * Wait for a shell command to exit with the expected code
   */
  "wait.command"?: WaitCommandAction;
  /**
   * Wait for a file or directory to exist (optionally containing a
   * substring)
   */
  "wait.file"?: WaitFileAction;
  /**
   * Wait for an HTTP endpoint to return an accepted status
   */
  "wait.http"?: WaitHttpAction;
  /**
   * Wait for a TCP port to accept connections
   */
  "wait.port"?: WaitPortAction;
}

/**
 * Complete mooncake configuration
 */
export type MooncakeConfig = Step[];

export default MooncakeConfig;
