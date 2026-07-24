// Package config provides configuration management for Mini-Orca.
//
// Configuration is loaded from a YAML file (config.yaml) with the
// following structure:
//
//	models:
//	  active_provider: "lm-studio"
//	  providers:
//	    lm-studio:
//	      base_url: "http://127.0.0.1:1234"
//	  phases:
//	    planning:
//	      provider: "lm-studio"
//	      model: "qwen/qwen3-coder-30b"
//	      temperature: 0.3
//	    coding:
//	      provider: "lm-studio"
//	      model: "qwen/qwen3-coder-30b"
//	      temperature: 0.1
//
//	agents:
//	  planner:
//	    skills: ["solid_principles", "clean_code"]
//	  coder:
//	    skills: ["function_generation", "clean_code"]
//
//	server:
//	  port: 8080
//
// # Configuration Flow
//
//  1. Load() reads config.yaml from config/ or .mini-orca/
//  2. If not found, creates a default config with sensible settings
//  3. BuildRouter() creates a model.Router from the models config
//  4. Agent skills are loaded from the agents config
//
// # Overriding Defaults
//
// Any config value can be overridden by creating a config.yaml file
// with only the values you want to change. Unspecified values fall
// back to defaults.
//
// # Example
//
//	cfg, err := config.Load("")
//	if err != nil {
//	    log.Fatal(err)
//	}
//
//	router, err := cfg.BuildRouter()
package config
