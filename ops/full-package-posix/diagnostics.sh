# Sourced by the launcher. tee is a stream copier, never the parent of a service.
start_diagnostic_capture() {
	if [ -z "${AC_LOG_DIR:-}" ]; then
		if [ -n "${ARCHIVE_CENTER_DATA_DIR:-}" ]; then
			AC_LOG_DIR="$ARCHIVE_CENTER_DATA_DIR/logs"
		elif printf '%s' "${PREFIX:-}" | grep -qi com.termux; then
			AC_LOG_DIR="$HOME/.archive-center-2.0/logs"
		else
			AC_LOG_DIR="$(CDPATH= cd -- "$(dirname -- "$0")/.." && pwd -P)/.runtime/logs"
		fi
	fi
	export AC_LOG_DIR
	if ! mkdir -p "$AC_LOG_DIR"; then
		printf 'Diagnostic log directory unavailable: %s\n' "$AC_LOG_DIR" >&2
		return
	fi
	for diagnostic_name in launcher.log launcher.out.log launcher.err.log chromadb.out.log chromadb.err.log schema.log update.log; do
		if [ -f "$AC_LOG_DIR/$diagnostic_name" ]; then mv -f "$AC_LOG_DIR/$diagnostic_name" "$AC_LOG_DIR/$diagnostic_name.1" || true; fi
	done
	DIAGNOSTIC_PIPE="$AC_LOG_DIR/.launcher-pipe-$$"
	if ! mkfifo "$DIAGNOSTIC_PIPE" "$DIAGNOSTIC_PIPE.err"; then
		rm -f "$DIAGNOSTIC_PIPE" "$DIAGNOSTIC_PIPE.err"
		printf 'Diagnostic capture unavailable\n' >&2; return
	fi
	exec 3>&1 4>&2
	tee -a "$AC_LOG_DIR/launcher.log" <"$DIAGNOSTIC_PIPE" >&3 &
	DIAGNOSTIC_TEE_PID=$!
	tee -a "$AC_LOG_DIR/launcher.log" <"$DIAGNOSTIC_PIPE.err" >&4 &
	DIAGNOSTIC_ERROR_TEE_PID=$!
	exec >"$DIAGNOSTIC_PIPE" 2>"$DIAGNOSTIC_PIPE.err"
	rm -f "$DIAGNOSTIC_PIPE" "$DIAGNOSTIC_PIPE.err"
	printf 'Diagnostic logs: %s\n' "$AC_LOG_DIR" >&2
}

finish_diagnostic_capture() {
	diagnostic_code=$1
	if [ -n "${DIAGNOSTIC_TEE_PID:-}" ]; then
		if [ "$diagnostic_code" -ne 0 ]; then
			printf 'Archive Center launcher stopped with exit code %s. Logs: %s\n' "$diagnostic_code" "$AC_LOG_DIR" >&2
			if [ -f "$AC_LOG_DIR/launcher.err.log" ]; then tail -n 25 "$AC_LOG_DIR/launcher.err.log" >&2 || true; fi
			printf 'Use 07_export_diagnostics.sh to save a report.\n' >&2
		fi
		exec 1>&3 2>&4
		# Child services are stopped by the existing cleanup before this call.
		wait "$DIAGNOSTIC_TEE_PID" || printf 'Diagnostic log copier failed\n' >&2
		wait "$DIAGNOSTIC_ERROR_TEE_PID" || printf 'Diagnostic error log copier failed\n' >&2
		DIAGNOSTIC_TEE_PID=
		DIAGNOSTIC_ERROR_TEE_PID=
	fi
}
