#!/usr/bin/env python3
"""Offline conformance checks for the audited thinking-schema runtime.

This is intentionally executable only with the candidate's pinned interpreter.
It imports the reviewed server source directly, replaces model access and token
generation, and uses FastAPI's in-process ASGI client without lifespan startup.
No weights, listener, or network request is involved.
"""

from __future__ import annotations

import argparse
import copy
import hashlib
import importlib
import importlib.metadata
import json
import os
import re
from pathlib import Path
from types import SimpleNamespace
from typing import Any


FIXED_REQUESTS = {
    "qwen38-v12-thinking-schema-1": {
        "model": "./models/qwen38-v12-thinking-schema-1",
        "temperature": 1.0,
        "max_tokens": 4096,
        "reasoning_effort": "low",
        "top_p": 0.95,
        "top_k": 20,
    },
    "qwen38-v12-medium-1": {
        "model": "./models/qwen38-v12-medium-1",
        "temperature": 1.0,
        "max_tokens": 4096,
        "reasoning_effort": "medium",
        "top_p": 0.95,
        "top_k": 20,
    },
}
FORBIDDEN_REQUEST_FIELDS = {"min_p", "presence_penalty", "repeat_penalty"}


def require_candidate(path: Path) -> dict[str, Any]:
    runtime = json.loads((path / "runtime.json").read_text(encoding="utf-8"))
    required_request = FIXED_REQUESTS.get(path.name)
    if required_request is None:
        raise AssertionError("candidate is not an approved fixed runtime profile")
    if runtime.get("wire_model") != required_request["model"] or runtime.get("request") != required_request:
        raise AssertionError("runtime request differs from the audited wire contract")
    if runtime.get("max_num_seqs") != 1:
        raise AssertionError("runtime does not pin one admission lane")
    environment = runtime.get("environment")
    if not isinstance(environment, dict) or environment.get("MAX_KV_SIZE") != "119552":
        raise AssertionError("runtime does not pin the required context limit")
    if environment.get("MLX_VLM_MAX_NUM_SEQS") != "1":
        raise AssertionError("runtime does not pin one server sequence")
    if environment.get("MLX_VLM_ENABLE_THINKING") != "1":
        raise AssertionError("runtime does not enable thinking")
    return runtime


def installed_runtime_checks(candidate_dir: Path, runtime_config: dict[str, Any]) -> None:
    import mlx_vlm
    from mlx_vlm import structured
    from mlx_vlm.server import generation, openai, request_normalization, schemas

    package_root = candidate_dir / "venv/lib/python3.11/site-packages/mlx_vlm"
    if Path(mlx_vlm.__file__).resolve().parent != package_root.resolve():
        raise AssertionError("mlx_vlm was not imported from the candidate's installed venv")
    if Path(structured.__file__).resolve().parent != package_root.resolve():
        raise AssertionError("structured grammar was not imported from the candidate's installed venv")
    expected_hashes = runtime_config.get("server_source_sha256")
    if not isinstance(expected_hashes, dict):
        raise AssertionError("candidate is missing the installed-server hash inventory")
    modules = {
        "schemas.py": schemas, "request_normalization.py": request_normalization,
        "app.py": importlib.import_module("mlx_vlm.server.app"), "openai.py": openai, "generation.py": generation,
        "runtime.py": importlib.import_module("mlx_vlm.server.runtime"),
    }
    for name, module in modules.items():
        path = Path(module.__file__).resolve()
        if path != (package_root / "server" / name).resolve():
            raise AssertionError(f"{name} was not imported from the installed candidate runtime")
        digest = hashlib.sha256(path.read_bytes()).hexdigest()
        if digest != expected_hashes.get(name):
            raise AssertionError(f"installed candidate hash drifted for {name}")
    if importlib.metadata.version("mlx-vlm") != runtime_config["distributions"]["mlx-vlm"]:
        raise AssertionError("installed mlx-vlm version drifted from the candidate lock")


def production_schema(source_root: Path) -> str:
    source = (source_root / "internal/app/file_analysis.go").read_text(encoding="utf-8")
    match = re.search(r"const fileAnalysisResponseSchemaDocument = `([^`]+)`", source)
    if match is None:
        raise AssertionError("production response schema literal was not found")
    return match.group(1)


def schema_checks(schema_text: str, tokenizer: Any) -> None:
    import llguidance as llg
    import llguidance.hf

    grammar = llg.JsonCompiler(separators=(", ", ": "), whitespace_pattern="").compile(schema_text)
    llg_tokenizer = llguidance.hf.from_tokenizer(tokenizer)
    error, warnings = llg.LLMatcher.validate_grammar_with_warnings(grammar, llg_tokenizer)
    if error or warnings:
        raise AssertionError(f"production grammar is not usable: {error!r} {warnings!r}")

    base = {
        "purpose": "Summarize a synthetic file.",
        "responsibilities": [], "dependencies": [], "side_effects": [],
        "risks": [], "suggestions": [], "symbol_explanations": {},
    }
    insight = {
        "mechanism": "Synthetic mechanism.",
        "why_it_matters_here": "Synthetic local evidence.",
        "tradeoff_or_failure_mode": "Synthetic constraint.",
        "transferable_lesson": "Synthetic measurement.",
    }
    cases: list[tuple[str, object, bool]] = [("omitted", base, True)]
    value = copy.deepcopy(base); value["engineering_insight"] = None; cases.append(("null", value, True))
    value = copy.deepcopy(base); value["engineering_insight"] = insight; cases.append(("full", value, True))
    value = copy.deepcopy(base); value["suggestions"] = [{"title": "Title", "summary": "Summary", "action": "Use \\\"quoted\\\" text at C:\\\\tmp\\\\x.\nThen check.", "engineering_insight": insight}]; cases.append(("escaped nested", value, True))
    value = copy.deepcopy(base); value["engineering_insight"] = {"mechanism": "partial"}; cases.append(("partial", value, False))
    value = copy.deepcopy(base); value["engineering_insight"] = {**insight, "extra": "x"}; cases.append(("extra", value, False))
    value = copy.deepcopy(base); value["engineering_insight"] = "wrong"; cases.append(("malformed type", value, False))
    value = copy.deepcopy(base); value["suggestions"] = [{"title": "Title", "summary": "Summary", "action": "BAD_ESCAPE"}]; cases.append(("malformed escape", json.dumps(value).replace("BAD_ESCAPE", r"\q"), False))
    for name, instance, expected in cases:
        text = instance if isinstance(instance, str) else json.dumps(instance, ensure_ascii=False)
        matcher = llg.LLMatcher(llg_tokenizer, grammar)
        tokens = llg_tokenizer.tokenize_str(text)
        accepted = matcher.validate_tokens(tokens) == len(tokens)
        if accepted:
            accepted = bool(matcher.consume_tokens(tokens) and matcher.is_accepting())
        if accepted != expected:
            raise AssertionError(f"schema grammar {name}: accepted={accepted}, expected={expected}")


class RecordingProcessor:
    def __init__(self) -> None:
        self.calls: list[int] = []
        self.resets = 0

    def process_last_token(self, token: int, logits):
        self.calls.append(token)
        return logits + 1

    def reset(self) -> None:
        self.resets += 1


def thinking_processor_checks(tokenizer: Any, schema_text: str) -> None:
    import mlx.core as mx
    from mlx_vlm.server.generation import GenerationArguments, ResponseGenerator
    from mlx_vlm.structured import ThinkingAwareLogitsProcessor, build_json_schema_logits_processor

    start = tokenizer.encode("<think>", add_special_tokens=False)[-1]
    end = tokenizer.encode("</think>", add_special_tokens=False)[-1]
    other = tokenizer.encode("x", add_special_tokens=False)[-1]
    logits = mx.zeros((1, 8))

    disabled = RecordingProcessor()
    ThinkingAwareLogitsProcessor(disabled, tokenizer, enable_thinking=False).process_last_token(other, logits)
    if disabled.calls != [other]:
        raise AssertionError("thinking-disabled grammar did not activate immediately")

    active = RecordingProcessor()
    wrapped = ThinkingAwareLogitsProcessor(active, tokenizer, enable_thinking=True)
    if wrapped.process_last_token(start, logits).tolist() != logits.tolist() or active.calls:
        raise AssertionError("grammar constrained an open thinking prefix")
    if wrapped.process_last_token(other, logits).tolist() != logits.tolist() or active.calls:
        raise AssertionError("grammar constrained reasoning before its close marker")
    if wrapped.process_last_token(end, logits).tolist() == logits.tolist() or active.calls != [end]:
        raise AssertionError("grammar did not activate at the thinking close marker")
    wrapped.reset()
    if active.resets != 1 or wrapped.process_last_token(other, logits).tolist() != logits.tolist():
        raise AssertionError("sequential request reused the prior thinking state")

    missing_end = ThinkingAwareLogitsProcessor(RecordingProcessor(), tokenizer, enable_thinking=True)
    missing_end.process_last_token(start, logits)
    missing_end.process_last_token(other, logits)
    if missing_end.processor.calls:
        raise AssertionError("grammar activated without a thinking close marker")

    closed_prefix = ThinkingAwareLogitsProcessor(RecordingProcessor(), tokenizer, enable_thinking=True)
    closed_prefix.process_last_token(end, logits)
    if closed_prefix.processor.calls != [end]:
        raise AssertionError("already-closed prefix did not activate the grammar")

    generator = ResponseGenerator.__new__(ResponseGenerator)
    generator.tokenizer = tokenizer
    grammar = RecordingProcessor()
    args = GenerationArguments(enable_thinking=True, logits_processors=[grammar], thinking_start_token="<think>", thinking_end_token="</think>")
    if not isinstance(generator._make_logits_processors(args, mx.array([start]))[0], ThinkingAwareLogitsProcessor):
        raise AssertionError("generation path did not wrap an open thinking prefix")
    if isinstance(generator._make_logits_processors(args, mx.array([start, end]))[0], ThinkingAwareLogitsProcessor):
        raise AssertionError("generation path wrapped an already-closed thinking prefix")

    compiled = build_json_schema_logits_processor(tokenizer, schema_text)
    compiled_wrapper = ThinkingAwareLogitsProcessor(compiled, tokenizer, enable_thinking=True)
    vocab_size = int(tokenizer.vocab_size)
    candidate_logits = mx.zeros((1, vocab_size))
    before_close = compiled_wrapper(mx.array([[start]]), candidate_logits)
    if not mx.array_equal(before_close, candidate_logits).item():
        raise AssertionError("compiled grammar constrained the open thinking token")
    after_close = compiled_wrapper(mx.array([[start, end]]), candidate_logits)
    allowed = tokenizer.encode('{"', add_special_tokens=False)[-1]
    disallowed = tokenizer.encode("[", add_special_tokens=False)[-1]
    if not mx.isfinite(after_close[0, allowed]).item() or mx.isfinite(after_close[0, disallowed]).item():
        raise AssertionError("compiled grammar did not allow the object prefix and reject an array after thinking")
    compiled_wrapper.reset()
    reset_open = compiled_wrapper(mx.array([[start]]), candidate_logits)
    if not mx.array_equal(reset_open, candidate_logits).item():
        raise AssertionError("compiled grammar retained state after reset")
    reset_closed = compiled_wrapper(mx.array([[start, end]]), candidate_logits)
    if not mx.isfinite(reset_closed[0, allowed]).item() or mx.isfinite(reset_closed[0, disallowed]).item():
        raise AssertionError("compiled grammar did not restart after reset")

    from mlx_vlm.server.generation import ThinkingBudgetCriteria
    budget = ThinkingBudgetCriteria(tokenizer, thinking_budget=1, thinking_end_token="</think>", thinking_start_token="<think>", enable_thinking=True, prompt_preopens_thinking=True)
    budget(start)
    if budget(other) is not None or budget(other) != tokenizer.encode("\n", add_special_tokens=False)[-1] or budget(other) != end:
        raise AssertionError("thinking budget did not force a close marker")


def engine_checks(tokenizer: Any, request: dict[str, Any]) -> None:
    from queue import Queue

    import mlx.core as mx
    from mlx_vlm.generate.ar import GenerationBatch
    from mlx_vlm.server import generation

    template_args = generation.GenerationArguments(max_tokens=request["max_tokens"], temperature=request["temperature"], top_p=request["top_p"], top_k=request["top_k"], enable_thinking=True, reasoning_effort=request["reasoning_effort"])
    rendered: dict[str, str] = {}
    for effort in ("low", "medium", "xhigh"):
        arguments = generation.GenerationArguments(max_tokens=request["max_tokens"], temperature=request["temperature"], top_p=request["top_p"], top_k=request["top_k"], enable_thinking=True, reasoning_effort=effort)
        kwargs = arguments.to_template_kwargs()
        if kwargs.get("reasoning_effort") != effort:
            raise AssertionError(f"local tokenizer arguments did not preserve {effort!r} reasoning")
        rendered[effort] = tokenizer.apply_chat_template([{"role": "user", "content": "synthetic"}], tokenize=False, add_generation_prompt=True, **kwargs)
    prompt = rendered[request["reasoning_effort"]]
    if not prompt.endswith("<think>\n"):
        raise AssertionError("local tokenizer template did not apply the selected thinking prompt")
    if request["reasoning_effort"] == "low" and "moving directly to the conclusion without unnecessary elaboration." not in prompt:
        raise AssertionError("local tokenizer template did not apply the historical low-effort thinking prompt")
    if request["reasoning_effort"] == "medium" and (hashlib.sha256(prompt.encode()).hexdigest() != "494e280281307944033f74025f48cddd84b0d3d0a1d756842e70b861b6f6641b" or len(set(rendered.values())) != 3):
        raise AssertionError("local tokenizer did not apply the fixed medium-reasoning template distinct from low and xhigh")

    sampler_owner = generation.ResponseGenerator.__new__(generation.ResponseGenerator)
    sampler = sampler_owner._make_sampler(template_args)
    logits = mx.arange(32, dtype=mx.float32)
    top_k_logits = sampler._apply_top_k(logits)
    if mx.sum(mx.isfinite(top_k_logits)).item() != request["top_k"] or mx.isfinite(top_k_logits[0]).item() or not mx.isfinite(top_k_logits[-1]).item():
        raise AssertionError("installed top-k sampler did not mask all but the configured 20 candidates")
    if generation.get_max_num_seqs() != 1:
        raise AssertionError("installed server did not read the configured one-sequence limit")

    raw_batch = GenerationBatch.__new__(GenerationBatch)
    raw_batch.uids = ["synthetic"]
    raw_batch.max_tokens = [request["max_tokens"]]
    raw_batch._num_tokens = [request["max_tokens"] - 1]
    raw_batch.stop_criteria = lambda _token: False
    raw_batch.thinking_budget_criteria = []
    raw_batch.token_context = []
    raw_batch.logits_processors = []
    raw_batch.prompt_cache = []
    raw_batch._current_tokens = raw_batch._current_lps = None
    raw_batch._next_tokens = raw_batch._next_lps = None
    raw_batch._next_top_idx = raw_batch._next_top_lp = None
    raw_batch._rope_deltas = None
    raw_batch._eval_pending_state = lambda: None
    raw_token = tokenizer.encode("reasoning", add_special_tokens=False)[-1]
    raw_batch._step = lambda: ([raw_token], [0.0], None, None)
    responses = raw_batch.next()
    if len(responses) != 1 or responses[0].token != raw_token or responses[0].finish_reason != "length" or raw_batch.uids:
        raise AssertionError("installed autoregressive engine did not stop at the raw 4096-token boundary")
    if raw_batch.next():
        raise AssertionError("installed autoregressive engine emitted a token after the raw cap")

    class FakeBatchGenerator:
        instances: list[Any] = []

        def __init__(self, *_args, **_kwargs) -> None:
            self.max_tokens = 0
            self.steps = 0
            self.unprocessed_prompts: list[Any] = []
            FakeBatchGenerator.instances.append(self)

        @property
        def has_work(self) -> bool:
            return False

        def insert(self, _prompts, *, max_tokens, **_kwargs):
            self.max_tokens = max_tokens
            return ("synthetic",)

        def next(self, **_kwargs):
            self.steps += 1
            finish_reason = "length" if self.steps == request["max_tokens"] else None
            if finish_reason:
                engine._stop = True
            response = SimpleNamespace(uid="synthetic", token=1, token_logprob=0.0, finish_reason=finish_reason)
            return [], [response]

        def close(self) -> None:
            pass

        def remove(self, _uid) -> None:
            pass

    class FakeStreamer:
        def __init__(self, *_args, **_kwargs) -> None:
            pass

        def advance(self, _token, _finish_reason) -> str:
            return ""

    engine = generation.ResponseGenerator.__new__(generation.ResponseGenerator)
    engine._stop = False
    engine.requests = Queue()
    engine._initialize_model = lambda: None
    engine._ready = SimpleNamespace(set=lambda: None)
    engine.model = SimpleNamespace(language_model=object())
    engine.processor = SimpleNamespace(tokenizer=tokenizer)
    engine.tokenizer = tokenizer
    engine.stop_tokens = set()
    engine.draft_model = engine.draft_kind = None
    engine.kv_bits = engine.kv_key_bits = engine.kv_value_bits = None
    engine.kv_key_scheme = engine.kv_value_scheme = None
    engine.kv_group_size = engine.quantized_kv_start = engine.top_logprobs_k = 0
    engine.kv_quant_scheme = "none"
    engine.apc_manager = engine.apc_mode = None
    engine.prefill_step_size = 1
    engine._gpu_embed = lambda raw_inputs, _images, apc_semantic_hash=None: (raw_inputs["input_ids"], {})
    engine._make_logits_processors = lambda _args, _input_ids: []
    engine._drain_cancellations = lambda: set()
    engine._log_prefill_started = lambda _request, **_kwargs: {}
    engine._log_decode_progress = lambda *_args, **_kwargs: 0.0
    output = Queue()
    engine.requests.put(generation.QueuedGenerationRequest(
        rqueue=output,
        raw_inputs={"input_ids": mx.array([[1]])},
        prompt_tokens=1,
        args=template_args,
        thinking_budget_criteria=None,
        request_id="synthetic",
        queued_at=0.0,
    ))
    original_batch_generator = generation.BatchGenerator
    original_streamer = generation._ServerTokenStreamer
    original_detokenizer = generation.make_streaming_detokenizer
    try:
        generation.BatchGenerator = FakeBatchGenerator
        generation._ServerTokenStreamer = FakeStreamer
        generation.make_streaming_detokenizer = lambda _processor: None
        engine._run_impl()
    finally:
        generation.BatchGenerator = original_batch_generator
        generation._ServerTokenStreamer = original_streamer
        generation.make_streaming_detokenizer = original_detokenizer
    if len(FakeBatchGenerator.instances) != 1 or FakeBatchGenerator.instances[0].max_tokens != request["max_tokens"]:
        raise AssertionError("actual engine did not pass the raw 4096-token cap to its batch generator")
    chunks = []
    while not output.empty():
        item = output.get_nowait()
        if isinstance(item, generation.StreamingToken):
            chunks.append(item)
    if len(chunks) != request["max_tokens"] or chunks[-1].finish_reason != "length" or sum(chunk.token_count for chunk in chunks) != request["max_tokens"]:
        raise AssertionError("actual engine did not stop the synthetic raw stream at the 4096-token boundary")

    admission = generation.ResponseGenerator.__new__(generation.ResponseGenerator)
    admission.requests, admission._stop = Queue(), False
    admission.requests.put("first")
    admission.requests.put("second")
    pending, should_stop = admission._collect_pending_requests(active=True, capacity=generation.get_max_num_seqs())
    if pending != ["first"] or should_stop or admission.requests.get_nowait() != "second":
        raise AssertionError("configured one-sequence admission did not retain the second request")


def route_checks(schema_text: str, tokenizer: Any, fixed_request: dict[str, Any]) -> None:
    from queue import Queue
    from fastapi.testclient import TestClient
    server = importlib.import_module("mlx_vlm.server.app")
    from mlx_vlm.server import openai
    import mlx.core as mx
    from mlx_vlm.server.generation import PromptTooLongError, ResponseGenerator, StreamingToken, _check_configured_context_budget
    from mlx_vlm.server.runtime import runtime
    from mlx_vlm.structured import ThinkingAwareLogitsProcessor

    class FakeGenerator:
        def __init__(self) -> None:
            self.calls: list[Any] = []
            self.active_processors: list[Any] = []
            self.finish_reason = "stop"

        def generate(self, *, prompt, images, audio, args, **_):
            self.calls.append(args)
            generator = ResponseGenerator.__new__(ResponseGenerator)
            generator.tokenizer = tokenizer
            generator.vision_cache = None
            generator.active_processors = generator._make_logits_processors(args, mx.array([tokenizer.encode("<think>", add_special_tokens=False)[-1]]))
            self.active_processors = generator.active_processors
            text = '<think>private reasoning</think>{"purpose":"Synthetic final.","responsibilities":[],"dependencies":[],"side_effects":[],"risks":[],"suggestions":[],"symbol_explanations":{}}'
            def tokens():
                yield StreamingToken(text=text, token=1, logprobs=0, finish_reason=self.finish_reason, token_count=12)
            return SimpleNamespace(prompt_tokens=4), tokens()

    fake = FakeGenerator()
    processor = SimpleNamespace(tokenizer=tokenizer, config=SimpleNamespace(temperature=1.0, top_p=0.95, top_k=20))
    server.get_cached_model = lambda *_args, **_kwargs: (object(), processor, SimpleNamespace())
    server.apply_chat_template = lambda *_args, **_kwargs: "<think>"
    runtime.response_generator = fake
    runtime.metrics.reset()
    # The route holds bound compatibility aliases; update them only after the
    # real app has registered its actual endpoint.
    openai.get_cached_model = lambda *_args, **_kwargs: (object(), processor, SimpleNamespace())
    openai.apply_chat_template = lambda *_args, **_kwargs: "<think>"
    request = {
        **fixed_request,
        "messages": [{"role": "user", "content": "synthetic"}],
        "response_format": {"type": "json_schema", "json_schema": {"name": "file_analysis", "strict": True, "schema": json.loads(schema_text)}},
    }
    # Creating TestClient without its context manager intentionally bypasses
    # ASGI lifespan startup; every route dependency above is a no-load stub.
    response = TestClient(server.app).post("/v1/chat/completions", json=request)
    if response.status_code != 200:
        raise AssertionError(f"actual route returned {response.status_code}: {response.text}")
    if len(fake.calls) != 1:
        raise AssertionError("actual route did not reach the stubbed generator exactly once")
    args = fake.calls[0]
    if args.max_tokens != fixed_request["max_tokens"] or args.temperature != fixed_request["temperature"] or args.reasoning_effort != fixed_request["reasoning_effort"] or args.top_p != fixed_request["top_p"] or args.top_k != fixed_request["top_k"]:
        raise AssertionError("actual route did not preserve audited sampling arguments")
    if not args.enable_thinking or len(args.logits_processors or []) != 1 or not isinstance(fake.active_processors[0], ThinkingAwareLogitsProcessor):
        raise AssertionError(f"actual route/generation did not wrap the compiled schema grammar for thinking: enabled={args.enable_thinking!r}, route_processors={[type(value).__name__ for value in args.logits_processors or []]}, active_processors={[type(value).__name__ for value in fake.active_processors]}")
    payload = response.json()
    message = payload["choices"][0]["message"]
    if message.get("content", "").startswith("<think>") or message.get("reasoning") != "private reasoning":
        raise AssertionError("actual route did not separate final content from reasoning")
    if payload["choices"][0]["finish_reason"] != "stop":
        raise AssertionError("stubbed non-truncated response was not reported as stop")
    if payload["usage"]["completion_tokens"] != 10 or payload["timings"]["predicted_n"] != 12:
        raise AssertionError("usage or raw timing did not preserve thinking delimiters correctly")
    if payload["timings"]["predicted_n"] > request["max_tokens"]:
        raise AssertionError("raw generated timing exceeded the requested completion cap")
    fake.finish_reason = "length"
    truncated = TestClient(server.app).post("/v1/chat/completions", json=request).json()
    if truncated["choices"][0]["finish_reason"] != "length":
        raise AssertionError("truncated completion was reported as successful")
    if set(request) & FORBIDDEN_REQUEST_FIELDS:
        raise AssertionError("test fixture accidentally sends forbidden request options")
    try:
        _check_configured_context_budget(119552 - 4096, 4096)
    except PromptTooLongError as error:
        raise AssertionError("configured context limit rejected its exact boundary") from error
    try:
        _check_configured_context_budget(119552 - 4096 + 1, 4096)
    except PromptTooLongError:
        pass
    else:
        raise AssertionError("configured context limit accepted an over-budget request")
    admission = ResponseGenerator.__new__(ResponseGenerator)
    admission.requests, admission._stop = Queue(), False
    admission.requests.put("first")
    admission.requests.put("second")
    pending, should_stop = admission._collect_pending_requests(active=True, capacity=1)
    if pending != ["first"] or should_stop or admission.requests.get_nowait() != "second":
        raise AssertionError("one-sequence admission did not retain the second request")


def main() -> None:
    parser = argparse.ArgumentParser()
    parser.add_argument("--candidate-dir", required=True, type=Path)
    parser.add_argument("--source-root", required=True, type=Path)
    args = parser.parse_args()
    os.environ.setdefault("HF_HUB_OFFLINE", "1")
    os.environ.setdefault("TRANSFORMERS_OFFLINE", "1")
    os.environ.setdefault("PYTHONDONTWRITEBYTECODE", "1")
    candidate_dir = args.candidate_dir.resolve()
    runtime_config = require_candidate(candidate_dir)
    runtime_environment = runtime_config["environment"]
    os.environ.update(runtime_environment)
    installed_runtime_checks(candidate_dir, runtime_config)
    from transformers import AutoTokenizer

    request = FIXED_REQUESTS[candidate_dir.name]
    model = candidate_dir / request["model"]
    tokenizer = AutoTokenizer.from_pretrained(model, local_files_only=True, trust_remote_code=False)
    schema_text = production_schema(args.source_root.resolve())
    schema_checks(schema_text, tokenizer)
    thinking_processor_checks(tokenizer, schema_text)
    engine_checks(tokenizer, request)
    route_checks(schema_text, tokenizer, request)
    print(json.dumps({"status": "passed", "generation_requests": 0, "model_weights_loaded": False, "route": "/v1/chat/completions"}, sort_keys=True))


if __name__ == "__main__":
    main()
