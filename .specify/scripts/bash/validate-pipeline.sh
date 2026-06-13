#!/usr/bin/env bash

# Pipeline checkpoint validation script
#
# Evaluates CP-1 through CP-10 gates for the AI Spec-to-SDK pipeline.
# See docs/ai-spec-to-sdk-pipeline.md for the full pipeline reference.
#
# Usage: ./validate-pipeline.sh [OPTIONS]
#
# OPTIONS:
#   --json              Output in JSON format
#   --spec <name>       Target spec directory name (e.g., 005-health)
#                       If omitted, derives from current branch
#   --stage <N>         Only check gates for stage N (1-7)
#   --skip-tests        Skip CP-5, CP-8, CP-9 (for spec-only validation)
#   --help, -h          Show help message
#
# CHECKPOINTS:
#   CP-1:  Constitution Check passes 10/10 in plan.md
#   CP-2:  Zero [NEEDS CLARIFICATION] markers in spec artifacts
#   CP-3:  FR traceability — every FR-* traces to downstream artifacts
#   CP-4:  Cross-spec consistency — imported types exist upstream
#   CP-5:  Upstream tests pass (go test on dependency packages)
#   CP-6:  Interface coverage — MUST-level FRs have Go implementations
#   CP-7:  Red phase — manual gate (tests fail before impl)
#   CP-8:  Green phase — go test on target package
#   CP-9:  Static analysis — go vet on target package
#   CP-10: Cross-language conformance — manual gate

set -e

# --- Argument Parsing ---

JSON_MODE=false
SPEC_NAME=""
STAGE=""
SKIP_TESTS=false
NEXT_ARG=""

for arg in "$@"; do
    if [[ -n "$NEXT_ARG" ]]; then
        case "$NEXT_ARG" in
            spec) SPEC_NAME="$arg" ;;
            stage) STAGE="$arg" ;;
        esac
        NEXT_ARG=""
        continue
    fi

    case "$arg" in
        --json)
            JSON_MODE=true
            ;;
        --spec)
            NEXT_ARG="spec"
            ;;
        --stage)
            NEXT_ARG="stage"
            ;;
        --skip-tests)
            SKIP_TESTS=true
            ;;
        --help|-h)
            sed -n '3,/^$/p' "$0" | sed 's/^# \?//'
            exit 0
            ;;
        *)
            echo "ERROR: Unknown option '$arg'. Use --help for usage information." >&2
            exit 1
            ;;
    esac
done

if [[ -n "$NEXT_ARG" ]]; then
    echo "ERROR: --$NEXT_ARG requires a value." >&2
    exit 1
fi

# --- Source Common Functions ---

SCRIPT_DIR="$(CDPATH="" cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
source "$SCRIPT_DIR/common.sh"

REPO_ROOT=$(get_repo_root)

# --- Resolve Spec Directory ---

if [[ -n "$SPEC_NAME" ]]; then
    SPEC_DIR="$REPO_ROOT/specs/$SPEC_NAME"
    if [[ ! -d "$SPEC_DIR" ]]; then
        echo "ERROR: Spec directory not found: $SPEC_DIR" >&2
        exit 1
    fi
else
    eval $(get_feature_paths)
    SPEC_DIR="$FEATURE_DIR"
    SPEC_NAME="$(basename "$SPEC_DIR")"
fi

# Derive spec number (first 3 digits)
SPEC_NUM=""
if [[ "$SPEC_NAME" =~ ^([0-9]{3})- ]]; then
    SPEC_NUM="${BASH_REMATCH[1]}"
fi

# --- Lookup Functions (bash 3 compatible) ---

# Spec number → Go package name
spec_to_pkg() {
    case "$1" in
        002) echo "keylib" ;;
        001) echo "account" ;;
        004) echo "topic" ;;
        003) echo "registry" ;;
        005) echo "health" ;;
        *)   echo "" ;;
    esac
}

# Spec number → upstream spec numbers (space-separated)
spec_deps() {
    case "$1" in
        002) echo "" ;;
        001) echo "002" ;;
        004) echo "001 002" ;;
        003) echo "004 001 002" ;;
        005) echo "003 004 001 002" ;;
        007) echo "003 004 001 002" ;;
        006) echo "" ;;
        *)   echo "" ;;
    esac
}

# Spec number → own FR prefix regex pattern
# Returns a grep-compatible pattern that matches only FRs belonging to this spec
spec_fr_prefix() {
    case "$1" in
        001) echo "^FR-[0-9]" ;;
        002) echo "^FR-[0-9]" ;;
        003) echo "^FR-R" ;;
        004) echo "^FR-T" ;;
        005) echo "^FR-H" ;;
        006) echo "^FR-[AWVX]" ;;
        007) echo "^FR-C" ;;
        *)   echo "" ;;
    esac
}

# Spec number → upstream Go package names (space-separated)
get_upstream_pkgs() {
    local deps
    deps=$(spec_deps "$1")
    local pkgs=""
    for dep in $deps; do
        local pkg
        pkg=$(spec_to_pkg "$dep")
        if [[ -n "$pkg" ]]; then
            pkgs="$pkgs $pkg"
        fi
    done
    echo "$pkgs"
}

# --- Checkpoint Results (bash 3 compatible indexed arrays) ---

CP_COUNT=0
# Arrays indexed 0..N-1
# CP_ID_0, CP_NAME_0, CP_STATUS_0, CP_DETAIL_0, etc.

add_result() {
    eval "CP_ID_${CP_COUNT}=\"\$1\""
    eval "CP_NAME_${CP_COUNT}=\"\$2\""
    eval "CP_STATUS_${CP_COUNT}=\"\$3\""
    eval "CP_DETAIL_${CP_COUNT}=\"\$4\""
    CP_COUNT=$((CP_COUNT + 1))
}

get_cp_id()     { eval echo "\$CP_ID_${1}"; }
get_cp_name()   { eval echo "\$CP_NAME_${1}"; }
get_cp_status() { eval echo "\$CP_STATUS_${1}"; }
get_cp_detail() { eval echo "\$CP_DETAIL_${1}"; }

# --- Stage-to-Checkpoint Mapping ---

should_run_cp() {
    local cp_num="$1"
    if [[ -z "$STAGE" ]]; then
        return 0  # Run all
    fi
    case "$STAGE" in
        1) [[ "$cp_num" == "1" || "$cp_num" == "2" ]] && return 0 ;;
        2) [[ "$cp_num" == "3" || "$cp_num" == "4" ]] && return 0 ;;
        3) [[ "$cp_num" == "5" || "$cp_num" == "6" ]] && return 0 ;;
        4) [[ "$cp_num" == "7" ]] && return 0 ;;
        5) [[ "$cp_num" == "8" || "$cp_num" == "9" ]] && return 0 ;;
        6) [[ "$cp_num" == "10" ]] && return 0 ;;
        7) return 1 ;;
    esac
    return 1
}

# --- Checkpoint Implementations ---

# CP-1: Constitution Check — plan.md has 10 passing principles
check_cp1() {
    local plan_file="$SPEC_DIR/plan.md"
    if [[ ! -f "$plan_file" ]]; then
        add_result "CP-1" "Constitution Check" "FAIL" "plan.md not found"
        return
    fi

    local pass_count
    # Match PASS variants in Constitution Check tables:
    #   **PASS**, PASS, PASS (reconciled), PASS (N/A for contracts), ✓, (N/A)
    pass_count=$(grep -c -E '\*\*PASS\*\*|\|\s*PASS(\s*\(|\s*\|)|\|\s*(N/A|✓)\s*\|' "$plan_file" 2>/dev/null || true)
    pass_count=${pass_count:-0}
    pass_count=$(echo "$pass_count" | tr -d '[:space:]')

    if [[ "$pass_count" -ge 10 ]]; then
        add_result "CP-1" "Constitution Check" "PASS" "${pass_count}/10 principles passing"
    else
        add_result "CP-1" "Constitution Check" "FAIL" "Only ${pass_count}/10 principles passing"
    fi
}

# CP-2: Zero NEEDS CLARIFICATION markers
check_cp2() {
    local marker_count=0
    local locations=""

    for file in spec.md plan.md data-model.md; do
        local filepath="$SPEC_DIR/$file"
        if [[ -f "$filepath" ]]; then
            local count
            # Exclude lines that describe the absence of markers (e.g., "No [NEEDS CLARIFICATION] markers")
            count=$(grep '\[NEEDS CLARIFICATION' "$filepath" 2>/dev/null | grep -cv 'No.*\[NEEDS CLARIFICATION\|no.*\[NEEDS CLARIFICATION\|0.*\[NEEDS CLARIFICATION' 2>/dev/null || true)
            count=${count:-0}
            count=$(echo "$count" | tr -d '[:space:]')
            if [[ "$count" -gt 0 ]]; then
                marker_count=$((marker_count + count))
                locations="$locations $file($count)"
            fi
        fi
    done

    if [[ "$marker_count" -eq 0 ]]; then
        add_result "CP-2" "Zero Ambiguity" "PASS" "0 markers found"
    else
        add_result "CP-2" "Zero Ambiguity" "FAIL" "${marker_count} markers in:${locations}"
    fi
}

# CP-3: FR Traceability
check_cp3() {
    local spec_file="$SPEC_DIR/spec.md"
    if [[ ! -f "$spec_file" ]]; then
        add_result "CP-3" "FR Traceability" "FAIL" "spec.md not found"
        return
    fi

    # Filter to own-spec FRs only (exclude cross-spec references)
    local fr_prefix
    fr_prefix=$(spec_fr_prefix "$SPEC_NUM")

    local frs
    if [[ -n "$fr_prefix" ]]; then
        frs=$(grep -oE 'FR-[A-Z]+-?[0-9]+|FR-[0-9]+' "$spec_file" 2>/dev/null | grep -E "$fr_prefix" | sort -u)
    else
        frs=$(grep -oE 'FR-[A-Z]+-?[0-9]+|FR-[0-9]+' "$spec_file" 2>/dev/null | sort -u)
    fi

    if [[ -z "$frs" ]]; then
        add_result "CP-3" "FR Traceability" "WARN" "No FR-* identifiers found in spec.md"
        return
    fi

    local total=0
    local traced=0
    local untraced=""

    local search_files=""
    [[ -f "$SPEC_DIR/data-model.md" ]] && search_files="$search_files $SPEC_DIR/data-model.md"
    [[ -f "$SPEC_DIR/tasks.md" ]] && search_files="$search_files $SPEC_DIR/tasks.md"
    if [[ -d "$SPEC_DIR/contracts" ]]; then
        for f in "$SPEC_DIR/contracts"/*.md; do
            [[ -f "$f" ]] && search_files="$search_files $f"
        done
    fi

    if [[ -z "$search_files" ]]; then
        add_result "CP-3" "FR Traceability" "WARN" "No downstream artifacts to check"
        return
    fi

    while IFS= read -r fr; do
        [[ -z "$fr" ]] && continue
        total=$((total + 1))
        if grep -ql "$fr" $search_files 2>/dev/null; then
            traced=$((traced + 1))
        else
            untraced="$untraced $fr"
        fi
    done <<< "$frs"

    if [[ "$traced" -eq "$total" ]]; then
        add_result "CP-3" "FR Traceability" "PASS" "${traced}/${total} FRs traced"
    elif [[ "$traced" -gt $((total * 80 / 100)) ]]; then
        add_result "CP-3" "FR Traceability" "WARN" "${traced}/${total} FRs traced"
    else
        add_result "CP-3" "FR Traceability" "FAIL" "${traced}/${total} FRs traced"
    fi
}

# CP-4: Cross-spec consistency
check_cp4() {
    local data_model="$SPEC_DIR/data-model.md"
    if [[ ! -f "$data_model" ]]; then
        add_result "CP-4" "Cross-Spec Consistency" "WARN" "data-model.md not found; skipped"
        return
    fi

    local deps
    deps=$(spec_deps "$SPEC_NUM")
    if [[ -z "$deps" ]]; then
        add_result "CP-4" "Cross-Spec Consistency" "PASS" "No upstream dependencies"
        return
    fi

    local issues=0
    local checked=0

    for dep_num in $deps; do
        local upstream_dir=""
        for dir in "$REPO_ROOT/specs/${dep_num}-"*; do
            if [[ -d "$dir" ]]; then
                upstream_dir="$dir"
                break
            fi
        done

        if [[ -n "$upstream_dir" && -f "$upstream_dir/data-model.md" ]]; then
            checked=$((checked + 1))
        elif grep -q "Spec ${dep_num}\|spec ${dep_num}" "$data_model" 2>/dev/null; then
            issues=$((issues + 1))
        fi
    done

    if [[ "$issues" -eq 0 ]]; then
        add_result "CP-4" "Cross-Spec Consistency" "PASS" "${checked} upstream specs verified"
    else
        add_result "CP-4" "Cross-Spec Consistency" "FAIL" "${issues} upstream data-models missing"
    fi
}

# CP-5: Upstream tests pass
check_cp5() {
    if $SKIP_TESTS; then
        add_result "CP-5" "Upstream Tests" "SKIP" "Skipped (--skip-tests)"
        return
    fi

    local upstream_pkgs
    upstream_pkgs=$(get_upstream_pkgs "$SPEC_NUM")

    if [[ -z "$upstream_pkgs" ]]; then
        add_result "CP-5" "Upstream Tests" "PASS" "No upstream packages"
        return
    fi

    local total=0
    local passed=0
    local failed_pkgs=""

    for pkg in $upstream_pkgs; do
        total=$((total + 1))
        local pkg_dir="$REPO_ROOT/impl/internal/$pkg"
        if [[ ! -d "$pkg_dir" ]]; then
            continue
        fi
        if (cd "$REPO_ROOT/impl" && go test "./internal/$pkg/..." > /dev/null 2>&1); then
            passed=$((passed + 1))
        else
            failed_pkgs="$failed_pkgs $pkg"
        fi
    done

    if [[ "$passed" -eq "$total" ]]; then
        add_result "CP-5" "Upstream Tests" "PASS" "${passed}/${total} packages pass"
    else
        add_result "CP-5" "Upstream Tests" "FAIL" "${passed}/${total} pass; failed:${failed_pkgs}"
    fi
}

# CP-6: Interface coverage
check_cp6() {
    local spec_file="$SPEC_DIR/spec.md"
    local target_pkg
    target_pkg=$(spec_to_pkg "$SPEC_NUM")

    if [[ -z "$target_pkg" ]]; then
        add_result "CP-6" "Interface Coverage" "WARN" "No Go package mapped for spec $SPEC_NUM"
        return
    fi

    local pkg_dir="$REPO_ROOT/impl/internal/$target_pkg"
    if [[ ! -d "$pkg_dir" ]]; then
        add_result "CP-6" "Interface Coverage" "WARN" "Package directory not found"
        return
    fi

    # Filter to own-spec FRs only (exclude cross-spec references)
    local fr_prefix
    fr_prefix=$(spec_fr_prefix "$SPEC_NUM")

    local frs
    if [[ -n "$fr_prefix" ]]; then
        frs=$(grep -oE 'FR-[A-Z]+-?[0-9]+|FR-[0-9]+' "$spec_file" 2>/dev/null | grep -E "$fr_prefix" | sort -u)
    else
        frs=$(grep -oE 'FR-[A-Z]+-?[0-9]+|FR-[0-9]+' "$spec_file" 2>/dev/null | sort -u)
    fi

    if [[ -z "$frs" ]]; then
        add_result "CP-6" "Interface Coverage" "WARN" "No FR-* identifiers found"
        return
    fi

    local total=0
    local covered=0

    while IFS= read -r fr; do
        [[ -z "$fr" ]] && continue
        total=$((total + 1))
        if grep -rql "$fr" "$pkg_dir/" 2>/dev/null; then
            covered=$((covered + 1))
        fi
    done <<< "$frs"

    if [[ "$covered" -eq "$total" ]]; then
        add_result "CP-6" "Interface Coverage" "PASS" "${covered}/${total} FRs covered in Go"
    elif [[ "$covered" -gt $((total * 80 / 100)) ]]; then
        add_result "CP-6" "Interface Coverage" "WARN" "${covered}/${total} FRs covered"
    else
        add_result "CP-6" "Interface Coverage" "FAIL" "${covered}/${total} FRs covered"
    fi
}

# CP-7: Red phase (manual gate)
check_cp7() {
    add_result "CP-7" "Red Phase" "MANUAL" "Verify tests fail before implementation"
}

# CP-8: Green phase
check_cp8() {
    if $SKIP_TESTS; then
        add_result "CP-8" "Green Phase" "SKIP" "Skipped (--skip-tests)"
        return
    fi

    local target_pkg
    target_pkg=$(spec_to_pkg "$SPEC_NUM")
    if [[ -z "$target_pkg" ]]; then
        add_result "CP-8" "Green Phase" "WARN" "No Go package mapped for spec $SPEC_NUM"
        return
    fi

    local pkg_dir="$REPO_ROOT/impl/internal/$target_pkg"
    if [[ ! -d "$pkg_dir" ]]; then
        add_result "CP-8" "Green Phase" "WARN" "Package directory not found"
        return
    fi

    if (cd "$REPO_ROOT/impl" && go test "./internal/$target_pkg/..." > /dev/null 2>&1); then
        add_result "CP-8" "Green Phase" "PASS" "go test passed"
    else
        add_result "CP-8" "Green Phase" "FAIL" "go test failed for $target_pkg"
    fi
}

# CP-9: Static analysis
check_cp9() {
    if $SKIP_TESTS; then
        add_result "CP-9" "Static Analysis" "SKIP" "Skipped (--skip-tests)"
        return
    fi

    local target_pkg
    target_pkg=$(spec_to_pkg "$SPEC_NUM")
    if [[ -z "$target_pkg" ]]; then
        add_result "CP-9" "Static Analysis" "WARN" "No Go package mapped for spec $SPEC_NUM"
        return
    fi

    local pkg_dir="$REPO_ROOT/impl/internal/$target_pkg"
    if [[ ! -d "$pkg_dir" ]]; then
        add_result "CP-9" "Static Analysis" "WARN" "Package directory not found"
        return
    fi

    if (cd "$REPO_ROOT/impl" && go vet "./internal/$target_pkg/..." > /dev/null 2>&1); then
        add_result "CP-9" "Static Analysis" "PASS" "go vet clean"
    else
        add_result "CP-9" "Static Analysis" "FAIL" "go vet warnings for $target_pkg"
    fi
}

# CP-10: Cross-language conformance (manual gate)
check_cp10() {
    local lang_count=0
    local langs=""

    [[ -d "$REPO_ROOT/impl/internal" ]] && lang_count=$((lang_count + 1)) && langs="${langs} Go"
    [[ -d "$REPO_ROOT/impl-rust" || -d "$REPO_ROOT/sdk-rust" ]] && lang_count=$((lang_count + 1)) && langs="${langs} Rust"
    [[ -d "$REPO_ROOT/impl-ts" || -d "$REPO_ROOT/sdk-ts" ]] && lang_count=$((lang_count + 1)) && langs="${langs} TypeScript"
    [[ -d "$REPO_ROOT/impl-py" || -d "$REPO_ROOT/sdk-py" ]] && lang_count=$((lang_count + 1)) && langs="${langs} Python"

    add_result "CP-10" "Cross-Language" "MANUAL" "${lang_count} language(s) detected:${langs}"
}

# --- Execute Checkpoints ---

run_if_needed() {
    local cp_num="$1"
    local fn="$2"
    if should_run_cp "$cp_num"; then
        "$fn"
    fi
}

run_if_needed 1  check_cp1
run_if_needed 2  check_cp2
run_if_needed 3  check_cp3
run_if_needed 4  check_cp4
run_if_needed 5  check_cp5
run_if_needed 6  check_cp6
run_if_needed 7  check_cp7
run_if_needed 8  check_cp8
run_if_needed 9  check_cp9
run_if_needed 10 check_cp10

# --- Compute Overall Status ---

auto_pass=0
auto_total=0
manual_count=0
skip_count=0
has_fail=false

i=0
while [[ "$i" -lt "$CP_COUNT" ]]; do
    status=$(get_cp_status "$i")
    case "$status" in
        PASS)   auto_pass=$((auto_pass + 1)); auto_total=$((auto_total + 1)) ;;
        FAIL)   auto_total=$((auto_total + 1)); has_fail=true ;;
        WARN)   auto_pass=$((auto_pass + 1)); auto_total=$((auto_total + 1)) ;;
        MANUAL) manual_count=$((manual_count + 1)) ;;
        SKIP)   skip_count=$((skip_count + 1)) ;;
    esac
    i=$((i + 1))
done

if $has_fail; then
    overall="FAIL"
elif [[ "$auto_pass" -eq "$auto_total" && "$auto_total" -gt 0 ]]; then
    overall="PASS"
else
    overall="PARTIAL"
fi

# --- Output ---

if $JSON_MODE; then
    printf '{"spec":"%s",' "$SPEC_NAME"
    if [[ -n "$STAGE" ]]; then
        printf '"stage":%s,' "$STAGE"
    else
        printf '"stage":"all",'
    fi
    printf '"checkpoints":['

    i=0
    while [[ "$i" -lt "$CP_COUNT" ]]; do
        [[ "$i" -gt 0 ]] && printf ','
        printf '{"id":"%s","name":"%s","status":"%s","detail":"%s"}' \
            "$(get_cp_id "$i")" \
            "$(get_cp_name "$i")" \
            "$(get_cp_status "$i")" \
            "$(get_cp_detail "$i")"
        i=$((i + 1))
    done

    printf '],'
    printf '"overall":"%s","automated_pass":%d,"automated_total":%d,"manual_gates":%d,"skipped":%d}\n' \
        "$overall" "$auto_pass" "$auto_total" "$manual_count" "$skip_count"
else
    echo ""
    echo "Pipeline Checkpoint Validation"
    echo "Spec: $SPEC_NAME | Stage: ${STAGE:-all}"
    echo ""

    i=0
    while [[ "$i" -lt "$CP_COUNT" ]]; do
        printf "%-6s %-26s %-8s (%s)\n" \
            "$(get_cp_id "$i")" \
            "$(get_cp_name "$i")" \
            "$(get_cp_status "$i")" \
            "$(get_cp_detail "$i")"
        i=$((i + 1))
    done

    echo ""
    echo "Overall: $overall (${auto_pass}/${auto_total} automated, ${manual_count} manual, ${skip_count} skipped)"
    echo ""
fi
