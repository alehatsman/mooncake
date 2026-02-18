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
export interface CommandAction {
  argv: string[];
  capture?: boolean;
  stdin?: string;
}

/**
 * Execute commands directly without shell interpolation
 * @category command
 */
export interface CommandActionAction {
  argv: string[];
  capture?: boolean;
  stdin?: string;
}

/**
 * Copy files with checksum verification and atomic writes
 * @category file
 */
export interface CopyAction {
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
export interface DownloadAction {
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
 * Manage files, directories, links, and permissions
 * @category file
 */
export interface FileAction {
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
   * Desired file state (present: file exists, absent: removed, directory:
   * dir exists, link: symlink, touch: update timestamp)
   * 
   * @values present | absent | directory | link | touch
   */
  state?: "present" | "absent" | "directory" | "link" | "touch";
}

/**
 * Delete text between start and end anchor patterns in files
 * @category file
 */
export interface FileDeleteRangeAction {
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
export interface FileInsertAction {
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
export interface FilePatchApplyAction {
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
export interface FileReplaceAction {
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
 * Include steps from another file
 */
export interface IncludeAction {
}

/**
 * Load variables from a YAML file
 * @category data
 */
export interface IncludeVarsAction {
}

/**
 * Manage system packages (install/remove/update)
 * 
 * @platforms linux, darwin, windows, freebsd
 * @requiresSudo true
 * @category system
 */
export interface PackageAction {
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
 * Execute a preset by expanding it into steps
 * @category system
 */
export interface PresetAction {
  name: string;
  with?: Record<string, any>;
}

/**
 * Display messages to the user
 * @category output
 */
export interface PrintAction {
  msg?: string;
}

/**
 * Apply multiple patches to multiple files atomically
 * @category file
 */
export interface RepoApplyPatchsetAction {
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
 * Manage services across platforms (systemd, launchd, Windows)
 * 
 * @platforms linux, darwin, windows
 * @requiresSudo true
 * @category system
 */
export interface ServiceAction {
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
 * Render template files and write to destination
 * @category file
 */
export interface TemplateAction {
  dest: string;
  mode?: string;
  src: string;
  vars?: Record<string, any>;
}

/**
 * Extract archive files (tar, tar.gz, zip) with path traversal protection
 * @category file
 */
export interface UnarchiveAction {
  creates?: string;
  dest: string;
  mode?: string;
  src: string;
  strip_components?: number;
}

/**
 * Define or update variables
 * @category data
 */
export interface VarsAction {
}

/**
 * Poll a condition until it becomes true or times out
 * @category system
 */
export interface WaitAction {
  allow_untracked?: boolean;
  cmd?: string;
  condition: string;
  exit_code?: number;
  host?: string;
  interval?: string;
  path?: string;
  port?: number;
  status?: number;
  timeout?: string;
  url?: string;
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
  creates?: string;
  /**
   * Skip step if this command succeeds (exit code 0). Useful for
   * idempotency (universal)
   */
  unless?: string;
  /**
   * Execute with sudo privileges. Works with: shell, command, file,
   * template
   */
  become?: boolean;
  /**
   * Tags for filtering step execution (universal)
   */
  tags?: string[];
  /**
   * Variable name to store step execution result (universal)
   */
  register?: string;
  /**
   * Directory path for iterating over files (universal)
   */
  with_filetree?: string;
  /**
   * Variable expression for iterating over items (universal)
   */
  with_items?: string;
  /**
   * Environment variables for the step
   */
  env?: Record<string, any>;
  /**
   * Working directory for the step
   */
  cwd?: string;
  /**
   * ⚠️ SHELL/COMMAND ONLY: Maximum execution time (e.g., '30s', '5m',
   * '1h'). Works with 'shell' and 'command' actions. Ignored for
   * file/template/include.
   */
  timeout?: string;
  /**
   * ⚠️ SHELL/COMMAND ONLY: Number of retry attempts on failure. Works
   * with 'shell' and 'command' actions. Ignored for file/template/include.
   */
  retries?: number;
  /**
   * ⚠️ SHELL/COMMAND ONLY: Delay between retry attempts (e.g., '1s',
   * '5s'). Works with 'shell' and 'command' actions. Ignored for
   * file/template/include.
   */
  retry_delay?: string;
  /**
   * Expression to override changed result
   */
  changed_when?: string;
  /**
   * Expression to override failure condition
   */
  failed_when?: string;
  /**
   * ⚠️ SHELL/COMMAND ONLY: User to become via sudo (e.g., 'root',
   * 'postgres'). Works with 'shell' and 'command' actions. Ignored for
   * file/template/include.
   */
  become_user?: string;
  /**
   * Path to YAML file with steps to include
   */
  include?: string;

  // Action fields (exactly one must be specified)
  /**
   * Capture file changes with enhanced metadata for LLM agents
   */
  artifact_capture?: ArtifactCaptureAction;
  /**
   * Validate artifacts against constraints (change budgets)
   */
  artifact_validate?: ArtifactValidateAction;
  /**
   * Verify conditions without changing system state
   */
  assert?: AssertAction;
  /**
   * Execute commands directly without shell interpolation
   */
  command?: CommandAction;
  /**
   * Copy files with checksum verification and atomic writes
   */
  copy?: CopyAction;
  /**
   * Download files from URLs with checksum verification
   */
  download?: DownloadAction;
  /**
   * Manage files, directories, links, and permissions
   */
  file?: FileAction;
  /**
   * Delete text between start and end anchor patterns in files
   */
  file_delete_range?: FileDeleteRangeAction;
  /**
   * Insert text before or after anchor patterns in files
   */
  file_insert?: FileInsertAction;
  /**
   * Apply unified diff patches to files
   */
  file_patch_apply?: FilePatchApplyAction;
  /**
   * Replace text in files using literal or regex patterns
   */
  file_replace?: FileReplaceAction;
  /**
   * Load variables from YAML files
   */
  include_vars?: IncludeVarsAction;
  /**
   * Manage system packages (install/remove/update)
   */
  package?: PackageAction;
  /**
   * Execute a preset by expanding it into steps
   */
  preset?: string | PresetAction;
  /**
   * Display messages to the user
   */
  print?: PrintAction;
  /**
   * Apply multiple patches to multiple files atomically
   */
  repo_apply_patchset?: RepoApplyPatchsetAction;
  /**
   * Search codebase for patterns and output results in JSON format
   */
  repo_search?: RepoSearchAction;
  /**
   * Generate a JSON representation of directory structure
   */
  repo_tree?: RepoTreeAction;
  /**
   * Manage services across platforms (systemd, launchd, Windows)
   */
  service?: ServiceAction;
  /**
   * Execute shell commands
   */
  shell?: string | ShellAction;
  /**
   * Render template files and write to destination
   */
  template?: TemplateAction;
  /**
   * Extract archive files (tar, tar.gz, zip) with path traversal
   * protection
   */
  unarchive?: UnarchiveAction;
  /**
   * Set variables for use in subsequent steps
   */
  vars?: VarsAction;
  /**
   * Poll a condition until it becomes true or times out
   */
  wait?: WaitAction;
}

/**
 * Complete mooncake configuration
 */
export type MooncakeConfig = Step[];

export default MooncakeConfig;
