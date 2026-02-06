# Mooncake Project Guide

**This project uses a universal LLM guide for all AI assistants.**

## 📖 Full Documentation

See **[LLM_GUIDE.md](./LLM_GUIDE.md)** for the complete project reference guide.

---

## Quick Context

**Mooncake** = Declarative config management tool (Go). "Docker for AI agents" - safe execution runtime with idempotency guarantees.

**Critical Rules**:
- ❌ **NEVER commit or push code** - user handles all git operations
- ✅ Focus on path resolution in presets (see LLM_GUIDE.md section)
- ✅ Follow definitive style guide: `docs/presets/definitive-style-guide.md`
- ✅ No over-engineering - minimal, focused solutions only

**Key Confusion Point**: Path resolution in presets
- Relative paths resolve from **including file's directory**
- Preset includes use `preset.BaseDir`
- See LLM_GUIDE.md "Path Expansion Summary" for details

**Architecture**: 5 core systems (Actions, Presets, Planner, Executor, Facts)
**Status**: Production-ready, 13 actions migrated ✅

For complete details, examples, and patterns → **[LLM_GUIDE.md](./LLM_GUIDE.md)**
