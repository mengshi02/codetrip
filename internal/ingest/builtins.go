package ingest

// Built-in function/method names that should not be tracked as call targets.
// Covers JS/TS, Python, Kotlin, C/C++, C#, PHP, Swift, Rust standard library functions.
// Go map does not allow duplicate keys; duplicates across languages are naturally deduplicated.

// BUILT_IN_NAMES is the set of built-in names to filter from call targets.
var BUILT_IN_NAMES = map[string]bool{
	// JavaScript/TypeScript
	"console": true, "log": true, "warn": true, "error": true, "info": true, "debug": true,
	"set":        true,
	"setTimeout": true, "setInterval": true, "clearTimeout": true, "clearInterval": true,
	"parseInt": true, "parseFloat": true, "isNaN": true, "isFinite": true,
	"encodeURI": true, "decodeURI": true, "encodeURIComponent": true, "decodeURIComponent": true,
	"JSON": true, "parse": true, "stringify": true,
	"Object": true, "Array": true, "String": true, "Number": true, "Boolean": true, "Symbol": true, "BigInt": true,
	"Map": true, "Set": true, "WeakMap": true, "WeakSet": true,
	"Promise": true, "resolve": true, "reject": true, "then": true, "catch": true, "finally": true,
	"Math": true, "Date": true, "RegExp": true, "Error": true,
	"require": true, "import": true, "export": true, "fetch": true, "Response": true, "Request": true,
	"useState": true, "useEffect": true, "useCallback": true, "useMemo": true, "useRef": true, "useContext": true,
	"useReducer": true, "useLayoutEffect": true, "useImperativeHandle": true, "useDebugValue": true,
	"createElement": true, "createContext": true, "createRef": true, "forwardRef": true, "memo": true, "lazy": true,
	"map": true, "filter": true, "reduce": true, "forEach": true, "find": true, "findIndex": true, "some": true, "every": true,
	"includes": true, "indexOf": true, "slice": true, "splice": true, "concat": true, "join": true, "split": true,
	"push": true, "pop": true, "shift": true, "unshift": true, "sort": true, "reverse": true,
	"keys": true, "values": true, "entries": true, "assign": true, "freeze": true, "seal": true,
	"hasOwnProperty": true, "toString": true, "valueOf": true,
	// Python (no overlap with JS above)
	"print": true, "len": true, "range": true, "str": true, "int": true, "float": true, "list": true, "dict": true, "tuple": true,
	"append": true, "extend": true, "update": true,
	"super": true, "type": true, "isinstance": true, "issubclass": true, "getattr": true, "setattr": true, "hasattr": true,
	"enumerate": true, "zip": true, "sorted": true, "reversed": true, "min": true, "max": true, "sum": true, "abs": true,
	// Kotlin stdlib (no overlap)
	"println": true, "readLine": true, "requireNotNull": true, "check": true, "assert": true,
	"listOf": true, "mapOf": true, "setOf": true, "mutableListOf": true, "mutableMapOf": true, "mutableSetOf": true,
	"arrayOf": true, "sequenceOf": true, "also": true, "apply": true, "run": true, "with": true, "takeIf": true, "takeUnless": true,
	"TODO": true, "buildString": true, "buildList": true, "buildMap": true, "buildSet": true,
	"repeat": true, "synchronized": true,
	"launch": true, "async": true, "runBlocking": true, "withContext": true, "coroutineScope": true,
	"supervisorScope": true, "delay": true,
	"flow": true, "flowOf": true, "collect": true, "emit": true, "onEach": true,
	"buffer": true, "conflate": true, "distinctUntilChanged": true,
	"flatMapLatest": true, "flatMapMerge": true, "combine": true,
	"stateIn": true, "shareIn": true, "launchIn": true,
	"to": true, "until": true, "downTo": true, "step": true,
	// C/C++ standard library (no overlap)
	"printf": true, "fprintf": true, "sprintf": true, "snprintf": true, "vprintf": true, "vfprintf": true, "vsprintf": true, "vsnprintf": true,
	"scanf": true, "fscanf": true, "sscanf": true,
	"malloc": true, "calloc": true, "realloc": true, "free": true, "memcpy": true, "memmove": true, "memset": true, "memcmp": true,
	"strlen": true, "strcpy": true, "strncpy": true, "strcat": true, "strncat": true, "strcmp": true, "strncmp": true, "strstr": true, "strchr": true, "strrchr": true,
	"atoi": true, "atol": true, "atof": true, "strtol": true, "strtoul": true, "strtoll": true, "strtoull": true, "strtod": true,
	"sizeof": true, "offsetof": true, "typeof": true,
	"abort": true, "exit": true, "_exit": true,
	"fopen": true, "fclose": true, "fread": true, "fwrite": true, "fseek": true, "ftell": true, "rewind": true, "fflush": true, "fgets": true, "fputs": true,
	"likely": true, "unlikely": true, "BUG": true, "BUG_ON": true, "WARN": true, "WARN_ON": true, "WARN_ONCE": true,
	"IS_ERR": true, "PTR_ERR": true, "ERR_PTR": true, "IS_ERR_OR_NULL": true,
	"ARRAY_SIZE": true, "container_of": true, "list_for_each_entry": true, "list_for_each_entry_safe": true,
	"clamp": true, "swap": true,
	"pr_info": true, "pr_warn": true, "pr_err": true, "pr_debug": true, "pr_notice": true, "pr_crit": true, "pr_emerg": true,
	"printk": true, "dev_info": true, "dev_warn": true, "dev_err": true, "dev_dbg": true,
	"GFP_KERNEL": true, "GFP_ATOMIC": true,
	"spin_lock": true, "spin_unlock": true, "spin_lock_irqsave": true, "spin_unlock_irqrestore": true,
	"mutex_lock": true, "mutex_unlock": true, "mutex_init": true,
	"kfree": true, "kmalloc": true, "kzalloc": true, "kcalloc": true, "krealloc": true, "kvmalloc": true, "kvfree": true,
	"get": true, "put": true,
	// C# / .NET built-ins (no overlap)
	"Console": true, "WriteLine": true, "ReadLine": true, "Write": true,
	"Task": true, "Run": true, "Wait": true, "WhenAll": true, "WhenAny": true, "FromResult": true, "Delay": true, "ContinueWith": true,
	"ConfigureAwait": true, "GetAwaiter": true, "GetResult": true,
	"ToString": true, "GetType": true, "Equals": true, "GetHashCode": true, "ReferenceEquals": true,
	"Add": true, "Remove": true, "Contains": true, "Clear": true, "Count": true, "Any": true, "All": true,
	"Where": true, "Select": true, "SelectMany": true, "OrderBy": true, "OrderByDescending": true, "GroupBy": true,
	"First": true, "FirstOrDefault": true, "Single": true, "SingleOrDefault": true, "Last": true, "LastOrDefault": true,
	"ToList": true, "ToArray": true, "ToDictionary": true, "AsEnumerable": true, "AsQueryable": true,
	"Aggregate": true, "Sum": true, "Average": true, "Distinct": true, "Skip": true, "Take": true,
	"Format": true, "IsNullOrEmpty": true, "IsNullOrWhiteSpace": true, "Concat": true,
	"Trim": true, "TrimStart": true, "TrimEnd": true, "Replace": true, "StartsWith": true, "EndsWith": true,
	"Convert": true, "ToInt32": true, "ToDouble": true, "ToBoolean": true, "ToByte": true,
	"Ceiling": true, "Floor": true, "Round": true, "Pow": true, "Sqrt": true,
	"Dispose": true, "Close": true,
	"TryParse": true,
	"AddRange": true, "RemoveAt": true, "RemoveAll": true, "FindAll": true, "Exists": true, "TrueForAll": true,
	"ContainsKey": true, "TryGetValue": true, "AddOrUpdate": true,
	"Throw": true, "ThrowIfNull": true,
	// PHP built-ins (no overlap)
	"echo": true, "isset": true, "empty": true, "unset": true, "compact": true, "extract": true,
	"count": true, "strpos": true, "strrpos": true, "substr": true, "strtolower": true, "strtoupper": true,
	"ltrim": true, "rtrim": true, "str_replace": true, "str_contains": true, "str_starts_with": true, "str_ends_with": true,
	"number_format": true,
	"array_map":     true, "array_filter": true, "array_reduce": true, "array_push": true, "array_pop": true, "array_shift": true,
	"array_unshift": true, "array_slice": true, "array_splice": true, "array_merge": true, "array_keys": true, "array_values": true,
	"array_key_exists": true, "in_array": true, "array_search": true, "array_unique": true, "usort": true, "rsort": true,
	"json_encode": true, "json_decode": true, "serialize": true, "unserialize": true,
	"intval": true, "floatval": true, "strval": true, "boolval": true, "is_null": true, "is_string": true, "is_int": true, "is_array": true,
	"is_object": true, "is_numeric": true, "is_bool": true, "is_float": true,
	"var_dump": true, "print_r": true, "var_export": true,
	"date": true, "time": true, "strtotime": true, "mktime": true, "microtime": true,
	"file_exists": true, "file_get_contents": true, "file_put_contents": true, "is_file": true, "is_dir": true,
	"preg_match": true, "preg_match_all": true, "preg_replace": true, "preg_split": true,
	"header": true, "session_start": true, "session_destroy": true, "ob_start": true, "ob_end_clean": true, "ob_get_clean": true,
	"dd": true, "dump": true,
	// Swift/iOS built-ins (no overlap)
	"debugPrint": true, "fatalError": true, "precondition": true, "preconditionFailure": true,
	"assertionFailure": true, "NSLog": true,
	"stride": true, "sequence": true, "repeatElement": true,
	"withUnsafePointer": true, "withUnsafeMutablePointer": true, "withUnsafeBytes": true,
	"autoreleasepool": true, "unsafeBitCast": true, "unsafeDowncast": true, "numericCast": true,
	"MemoryLayout": true,
	"flatMap":      true, "compactMap": true,
	"first": true, "last": true, "prefix": true, "suffix": true, "dropFirst": true, "dropLast": true,
	"enumerated": true, "joined": true,
	"removeAll": true, "removeFirst": true, "removeLast": true,
	"isEmpty": true, "index": true, "startIndex": true, "endIndex": true,
	"addSubview": true, "removeFromSuperview": true, "layoutSubviews": true, "setNeedsLayout": true,
	"layoutIfNeeded": true, "setNeedsDisplay": true, "invalidateIntrinsicContentSize": true,
	"addTarget": true, "removeTarget": true, "addGestureRecognizer": true,
	"addConstraint": true, "addConstraints": true, "removeConstraint": true, "removeConstraints": true,
	"NSLocalizedString": true, "Bundle": true,
	"reloadData": true, "reloadSections": true, "reloadRows": true, "performBatchUpdates": true,
	"register": true, "dequeueReusableCell": true, "dequeueReusableSupplementaryView": true,
	"beginUpdates": true, "endUpdates": true, "insertRows": true, "deleteRows": true, "insertSections": true, "deleteSections": true,
	"present": true, "dismiss": true, "pushViewController": true, "popViewController": true, "popToRootViewController": true,
	"performSegue": true, "prepare": true,
	"DispatchQueue": true, "sync": true, "asyncAfter": true,
	"withCheckedContinuation": true, "withCheckedThrowingContinuation": true,
	"sink": true, "store": true, "receive": true, "subscribe": true,
	"addObserver": true, "removeObserver": true, "post": true, "NotificationCenter": true,
	// Rust standard library (no overlap)
	"unwrap": true, "expect": true, "unwrap_or": true, "unwrap_or_else": true, "unwrap_or_default": true,
	"ok": true, "err": true, "is_ok": true, "is_err": true, "map_err": true, "and_then": true, "or_else": true,
	"clone": true, "to_string": true, "to_owned": true, "into": true, "from": true, "as_ref": true, "as_mut": true,
	"iter": true, "into_iter": true, "fold": true, "for_each": true,
	"is_empty": true, "format": true, "writeln": true, "panic": true, "unreachable": true, "todo": true, "unimplemented": true,
	"vec": true, "eprintln": true, "dbg": true,
	"lock": true, "try_lock": true,
	"spawn": true, "sleep": true,
	"Some": true, "None": true, "Ok": true, "Err": true,
}

// IsBuiltInOrNoise checks if a name is a built-in function or common noise that should be filtered out.
func IsBuiltInOrNoise(name string) bool {
	return BUILT_IN_NAMES[name]
}

// IsBuiltInOrNoiseForLanguage prevents one language's conventional runtime
// methods from suppressing repository-defined symbols in another language.
func IsBuiltInOrNoiseForLanguage(name, language string) bool {
	if language == "cpp" {
		switch name {
		case "ToString", "find", "run":
			return false
		}
	}
	return IsBuiltInOrNoise(name)
}
