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
  follow_symlinks?: boolean;
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
 * Switch an existing git working tree to a specified ref
 * @category system
 */
export interface GitCheckoutAction {
  dest: string;
  force?: boolean;
  ref: string;
}

/**
 * Idempotently clone or update a git repository at a specific ref
 * @category network
 */
export interface GitCloneAction {
  credentials?: {
    password: string;
    ssh_key: string;
    ssh_options: string;
    username: string;
  };
  depth?: number;
  dest: string;
  force?: boolean;
  recurse_submodules?: boolean;
  ref?: string;
  repo: string;
  update?: boolean;
  url?: string;
}

/**
 * Idempotently manage git config keys at local, global, or system scope
 * @category system
 */
export interface GitConfigAction {
  dest?: string;
  repo?: string;
  scope: string;
  set?: Record<string, any>;
  unset?: string[];
}

/**
 * Issue an HTTP request; capture the response as a registered fact
 * @category network
 */
export interface HttpRequestAction {
  auth?: {
    basic: {
    pass: string;
    user: string;
  };
    bearer: string;
    header: {
    name: string;
    value: string;
  };
  };
  body?: string;
  creates_when?: string;
  expect_status?: number[];
  file?: string;
  follow_redirects?: number;
  form?: Record<string, any>;
  headers?: Record<string, any>;
  idempotency_key?: string;
  json?: any;
  max_response_bytes?: number;
  method?: string;
  probe?: {
    auth: {
    basic: {
    pass: string;
    user: string;
  };
    bearer: string;
    header: {
    name: string;
    value: string;
  };
  };
    body: string;
    creates_when: string;
    expect_status: number[];
    file: string;
    follow_redirects: number;
    form: Record<string, any>;
    headers: Record<string, any>;
    idempotency_key: string;
    json: any;
    max_response_bytes: number;
    method: string;
    probe: Record<string, any>;
    redact_body: boolean;
    retries: number;
    retry_delay: string;
    retry_on: string[];
    reverse: Record<string, any>;
    risk: string;
    save_to: string;
    skip_tls_verify: boolean;
    timeout: string;
    url: string;
  };
  redact_body?: boolean;
  retries?: number;
  retry_delay?: string;
  retry_on?: string[];
  reverse?: {
    auth: {
    basic: {
    pass: string;
    user: string;
  };
    bearer: string;
    header: {
    name: string;
    value: string;
  };
  };
    body: string;
    creates_when: string;
    expect_status: number[];
    file: string;
    follow_redirects: number;
    form: Record<string, any>;
    headers: Record<string, any>;
    idempotency_key: string;
    json: any;
    max_response_bytes: number;
    method: string;
    probe: Record<string, any>;
    redact_body: boolean;
    retries: number;
    retry_delay: string;
    retry_on: string[];
    reverse: Record<string, any>;
    risk: string;
    save_to: string;
    skip_tls_verify: boolean;
    timeout: string;
    url: string;
  };
  risk?: string;
  save_to?: string;
  skip_tls_verify?: boolean;
  timeout?: string;
  url: string;
}

/**
 * Import steps from another file
 */
export interface ImportAction {
}

/**
 * Display messages and structured data to the user
 * @category output
 */
export interface LogAction {
  /**
   * Max bytes of rendered output; 0 disables truncation
   */
  budget?: number;
  /**
   * Structured payload to render
   */
  data?: any;
  /**
   * Render format for Data
   * 
   * @values kv | json
   * @default kv
   */
  format?: "kv" | "json";
  /**
   * Free-text message (supports templates)
   */
  msg?: string;
  /**
   * Optional header above Data (kv mode only)
   */
  title?: string;
}

/**
 * Single-shot read of CPU utilization + load averages
 * @category system
 */
export interface ObserveCpuAction {
}

/**
 * Single-shot read of filesystem space / inode usage for a path
 * @category system
 */
export interface ObserveDiskAction {
  path?: string;
}

/**
 * Single-shot read of GPU utilization + memory (NVIDIA/Apple)
 * @category system
 */
export interface ObserveGpuAction {
  index?: number;
}

/**
 * Single-shot HTTP GET; returns typed status, latency, headers, body sample
 * @category network
 */
export interface ObserveHttpAction {
  capture_headers?: string[];
  expect_status?: number;
  follow_redirects?: number;
  method?: string;
  skip_tls_verify?: boolean;
  timeout?: string;
  url: string;
}

/**
 * Single-shot read of a log source; returns per-pattern match counts + sample lines
 * @category system
 */
export interface ObserveLogsAction {
  container?: string;
  journal_unit?: string;
  max_bytes?: number;
  max_lines?: number;
  path?: string;
  patterns: string[];
  sample_lines?: number;
  since?: string;
}

/**
 * Single-shot read of memory + swap state (total/used/free/available)
 * @category system
 */
export interface ObserveMemoryAction {
}

/**
 * Single-shot read of TCP/UDP port state (open? listener? pid?)
 * @category network
 */
export interface ObservePortAction {
  host?: string;
  port: number;
  protocol?: string;
  timeout?: string;
}

/**
 * Single-shot read of process state (running? pid? args?)
 * @category system
 */
export interface ObserveProcessAction {
  name?: string;
  pattern?: string;
}

/**
 * Single-shot read of service state (active? enabled?)
 * @category system
 */
export interface ObserveServiceAction {
  manager?: string;
  name: string;
}

/**
 * Manage a cron job via /etc/cron.d/<name>
 * 
 * @platforms linux
 * @requiresSudo true
 * @category system
 */
export interface OsCronAction {
  command?: string;
  day?: string;
  env?: Record<string, any>;
  hour?: string;
  minute?: string;
  month?: string;
  name: string;
  schedule?: string;
  /**
   * 
   * @values present | absent
   */
  state?: "present" | "absent";
  user?: string;
  weekday?: string;
}

/**
 * Manage host firewall rules (ufw backend)
 * 
 * @platforms linux
 * @requiresSudo true
 * @category system
 */
export interface OsFirewallAction {
  backend?: string;
  rule?: {
    action: string;
    comment: string;
    from: string;
    port: number;
    protocol: string;
  };
  rules?: {
    action: string;
    comment: string;
    from: string;
    port: number;
    protocol: string;
  }[];
  state?: string;
}

/**
 * Declaratively manage a Unix group
 * 
 * @platforms linux, darwin
 * @requiresSudo true
 * @category system
 */
export interface OsGroupAction {
  gid?: number;
  name: string;
  state?: string;
  system?: boolean;
}

/**
 * Manage an /etc/fstab entry and the matching live mount
 * 
 * @platforms linux
 * @requiresSudo true
 * @category system
 */
export interface OsMountAction {
  backup?: boolean;
  dest: string;
  dump?: number;
  fstype?: string;
  options?: string[];
  pass?: number;
  src?: string;
  state?: string;
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
 * Manage authorized_keys entries for a user (idempotent per-key)
 * 
 * @platforms linux, darwin
 * @category system
 */
export interface OsSshKeyAction {
  exclusive?: boolean;
  key?: string;
  keys?: string[];
  options?: string[];
  path?: string;
  state?: string;
  user: string;
}

/**
 * Manage a Linux kernel parameter (sysctl)
 * 
 * @platforms linux
 * @requiresSudo true
 * @category system
 */
export interface OsSysctlAction {
  name: string;
  persist?: boolean;
  reload?: boolean;
  /**
   * 
   * @values present | absent
   */
  state?: "present" | "absent";
  value?: string | number | boolean;
}

/**
 * Manage a systemd unit file with daemon-reload, enable, start
 * 
 * @platforms linux
 * @requiresSudo true
 * @category system
 */
export interface OsSystemdAction {
  enabled?: boolean;
  install?: Record<string, any>;
  name: string;
  path?: string;
  reload_on_change?: boolean;
  service?: Record<string, any>;
  socket?: Record<string, any>;
  started?: boolean;
  state?: string;
  timer?: Record<string, any>;
  unit?: Record<string, any>;
}

/**
 * Declaratively manage a system user account
 * 
 * @platforms linux, darwin
 * @requiresSudo true
 * @category system
 */
export interface OsUserAction {
  append_groups?: boolean;
  comment?: string;
  create_home?: boolean;
  gid?: number;
  group?: string;
  groups?: string[];
  home?: string;
  name: string;
  remove_home?: boolean;
  shell?: string;
  state?: string;
  system?: boolean;
  uid?: number;
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
 * Mark or unmark packages as held to prevent upgrade/removal (apt on linux, brew on darwin)
 * 
 * @platforms linux, darwin
 * @requiresSudo true
 * @category system
 */
export interface PkgHoldAction {
  manager?: string;
  name?: string;
  names?: string[];
  state?: string;
}

/**
 * Return the installed packages and versions (read-only; apt on linux, brew on darwin)
 * 
 * @platforms linux, darwin
 * @category system
 */
export interface PkgListAction {
  manager?: string;
}

/**
 * Manage a third-party package repository (apt + brew taps; dnf deferred)
 * 
 * @platforms linux, darwin
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
 * Upgrade named packages or all installed packages (apt on linux, brew on darwin)
 * 
 * @platforms linux, darwin
 * @requiresSudo true
 * @category system
 */
export interface PkgUpgradeAction {
  autoremove?: boolean;
  manager?: string;
  names?: string[];
}

/**
 * Read a JSON file and optionally extract a value by path
 * @category data
 */
export interface ReadJsonAction {
  max_bytes?: number;
  path: string;
  query?: string;
  redact?: string[];
}

/**
 * Read a YAML file and optionally extract a value by path
 * @category data
 */
export interface ReadYamlAction {
  max_bytes?: number;
  path: string;
  query?: string;
  redact?: string[];
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
  creates?: string;
  /**
   * Windows + PowerShell only — sets $ErrorActionPreference (Stop,
   * Continue, SilentlyContinue, ...). Default: Stop. Ignored elsewhere.
   */
  error_action?: string;
  /**
   * Shell interpreter binary (any executable on PATH). Default: bash on
   * Unix, powershell on Windows. On Windows, 'cmd' dispatches with /c,
   * others with -Command.
   */
  interpreter?: string;
  /**
   * Windows only — assert the mooncake process is elevated; fail the
   * step if it isn't. Does not attempt UAC. Ignored on Unix.
   */
  run_as_admin?: boolean;
  /**
   * Input to provide to the command via stdin
   */
  stdin?: string;
  unless?: string;
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
  creates?: string;
  /**
   * Windows + PowerShell only — sets $ErrorActionPreference (Stop,
   * Continue, SilentlyContinue, ...). Default: Stop. Ignored elsewhere.
   */
  error_action?: string;
  /**
   * Shell interpreter binary (any executable on PATH). Default: bash on
   * Unix, powershell on Windows. On Windows, 'cmd' dispatches with /c,
   * others with -Command.
   */
  interpreter?: string;
  /**
   * Windows only — assert the mooncake process is elevated; fail the
   * step if it isn't. Does not attempt UAC. Ignored on Unix.
   */
  run_as_admin?: boolean;
  /**
   * Input to provide to the command via stdin
   */
  stdin?: string;
  unless?: string;
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
 * Ensure a line is present or absent in a file (lineinfile-equivalent)
 * @category file
 */
export interface TextLineAction {
  backup?: boolean;
  insert_after?: string;
  insert_before?: string;
  line?: string;
  path: string;
  regexp?: string;
  state?: string;
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
 * Apply structural section/key edits to an INI-style configuration file
 * @category file
 */
export interface TextPatchIniAction {
  backup?: boolean;
  delete?: string[];
  path: string;
  set?: Record<string, any>;
}

/**
 * Apply structural set/delete/merge edits to a JSON file with order + indent preservation
 * @category file
 */
export interface TextPatchJsonAction {
  backup?: boolean;
  delete?: string[];
  merge?: Record<string, any>;
  merge_strategy?: string;
  path: string;
  set?: Record<string, any>;
}

/**
 * Apply structural set/delete/merge edits to a YAML file, preserving order and adjacent comments
 * @category file
 */
export interface TextPatchYamlAction {
  backup?: boolean;
  delete?: string[];
  merge?: Record<string, any>;
  merge_strategy?: string;
  path: string;
  set?: Record<string, any>;
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
  interval?: string;
  poll_interval?: string;
  timeout?: string;
}

/**
 * Wait for a file or directory to exist (optionally containing a substring)
 * @category system
 */
export interface WaitFileAction {
  contains?: string;
  interval?: string;
  path: string;
  poll_interval?: string;
  timeout?: string;
}

/**
 * Wait for an HTTP endpoint to return an accepted status
 * @category network
 */
export interface WaitHttpAction {
  body?: string;
  body_contains?: string;
  headers?: Record<string, any>;
  interval?: string;
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
  interval?: string;
  poll_interval?: string;
  port: number;
  timeout?: string;
}

/**
 * Manage Windows Firewall inbound/outbound rules
 * 
 * @platforms windows
 * @category system
 */
export interface WindowsFirewallRuleAction {
  action?: string;
  description?: string;
  direction?: string;
  enabled?: boolean;
  local_port?: string[];
  name: string;
  profile?: string[];
  protocol?: string;
  remote_port?: string[];
  state?: string;
}

/**
 * Manage Windows Task Scheduler entries
 * 
 * @platforms windows
 * @category system
 */
export interface WindowsScheduledTaskAction {
  actions: {
    arguments: string;
    execute: string;
    working_directory: string;
  }[];
  description?: string;
  name: string;
  principal?: {
    logon_type: string;
    run_level: string;
    user: string;
  };
  settings?: {
    allow_start_if_on_batteries: boolean;
    dont_stop_if_going_on_batteries: boolean;
    execution_time_limit: string;
    hidden: boolean;
    multiple_instances: string;
    restart_count: number;
    restart_interval: string;
    run_only_if_network_available: boolean;
    start_when_available: boolean;
  };
  state?: string;
  trigger?: {
    duration: string;
    interval: string;
    start_boundary: string;
    type: string;
    user_id: string;
  };
  triggers?: {
    duration: string;
    interval: string;
    start_boundary: string;
    type: string;
    user_id: string;
  }[];
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
   * Iterate over items (universal). Accepts a template-variable string or
   * an inline list.
   */
  for_each?: string | any[];
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
   * Switch an existing git working tree to a specified ref
   */
  "git.checkout"?: GitCheckoutAction;
  /**
   * Idempotently clone or update a git repository at a specific ref
   */
  "git.clone"?: GitCloneAction;
  /**
   * Idempotently manage git config keys at local, global, or system scope
   */
  "git.config"?: GitConfigAction;
  /**
   * Issue an HTTP request; capture the response as a registered fact
   */
  "http.request"?: HttpRequestAction;
  /**
   * Display messages and structured data to the user
   */
  log?: LogAction;
  /**
   * Single-shot read of CPU utilization + load averages
   */
  "observe.cpu"?: ObserveCpuAction;
  /**
   * Single-shot read of filesystem space / inode usage for a path
   */
  "observe.disk"?: ObserveDiskAction;
  /**
   * Single-shot read of GPU utilization + memory (NVIDIA/Apple)
   */
  "observe.gpu"?: ObserveGpuAction;
  /**
   * Single-shot HTTP GET; returns typed status, latency, headers, body
   * sample
   */
  "observe.http"?: ObserveHttpAction;
  /**
   * Single-shot read of a log source; returns per-pattern match counts +
   * sample lines
   */
  "observe.logs"?: ObserveLogsAction;
  /**
   * Single-shot read of memory + swap state (total/used/free/available)
   */
  "observe.memory"?: ObserveMemoryAction;
  /**
   * Single-shot read of TCP/UDP port state (open? listener? pid?)
   */
  "observe.port"?: ObservePortAction;
  /**
   * Single-shot read of process state (running? pid? args?)
   */
  "observe.process"?: ObserveProcessAction;
  /**
   * Single-shot read of service state (active? enabled?)
   */
  "observe.service"?: ObserveServiceAction;
  /**
   * Manage a cron job via /etc/cron.d/<name>
   */
  "os.cron"?: OsCronAction;
  /**
   * Manage host firewall rules (ufw backend)
   */
  "os.firewall"?: OsFirewallAction;
  /**
   * Declaratively manage a Unix group
   */
  "os.group"?: OsGroupAction;
  /**
   * Manage an /etc/fstab entry and the matching live mount
   */
  "os.mount"?: OsMountAction;
  /**
   * Manage services across platforms (systemd, launchd, Windows)
   */
  "os.service"?: OsServiceAction;
  /**
   * Manage authorized_keys entries for a user (idempotent per-key)
   */
  "os.ssh_key"?: OsSshKeyAction;
  /**
   * Manage a Linux kernel parameter (sysctl)
   */
  "os.sysctl"?: OsSysctlAction;
  /**
   * Manage a systemd unit file with daemon-reload, enable, start
   */
  "os.systemd"?: OsSystemdAction;
  /**
   * Declaratively manage a system user account
   */
  "os.user"?: OsUserAction;
  /**
   * Manage system packages (install/remove/update)
   */
  pkg?: PkgAction;
  /**
   * Mark or unmark packages as held to prevent upgrade/removal (apt on
   * linux, brew on darwin)
   */
  "pkg.hold"?: PkgHoldAction;
  /**
   * Return the installed packages and versions (read-only; apt on linux,
   * brew on darwin)
   */
  "pkg.list"?: PkgListAction;
  /**
   * Manage a third-party package repository (apt + brew taps; dnf
   * deferred)
   */
  "pkg.repo"?: PkgRepoAction;
  /**
   * Upgrade named packages or all installed packages (apt on linux, brew
   * on darwin)
   */
  "pkg.upgrade"?: PkgUpgradeAction;
  /**
   * Read a JSON file and optionally extract a value by path
   */
  "read.json"?: ReadJsonAction;
  /**
   * Read a YAML file and optionally extract a value by path
   */
  "read.yaml"?: ReadYamlAction;
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
   * Ensure a line is present or absent in a file (lineinfile-equivalent)
   */
  "text.line"?: TextLineAction;
  /**
   * Apply unified diff patches to files
   */
  "text.patch"?: TextPatchAction;
  /**
   * Apply structural section/key edits to an INI-style configuration file
   */
  "text.patch.ini"?: TextPatchIniAction;
  /**
   * Apply structural set/delete/merge edits to a JSON file with order +
   * indent preservation
   */
  "text.patch.json"?: TextPatchJsonAction;
  /**
   * Apply structural set/delete/merge edits to a YAML file, preserving
   * order and adjacent comments
   */
  "text.patch.yaml"?: TextPatchYamlAction;
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
  /**
   * Manage Windows Firewall inbound/outbound rules
   */
  "windows.firewall_rule"?: WindowsFirewallRuleAction;
  /**
   * Manage Windows Task Scheduler entries
   */
  "windows.scheduled_task"?: WindowsScheduledTaskAction;
}

/**
 * Complete mooncake configuration
 */
export type MooncakeConfig = Step[];

export default MooncakeConfig;
