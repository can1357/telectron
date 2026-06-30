(() => {
				const Page_Function_toString = Function.prototype.toString;
				const Page_FunctionToStringDescriptor = Object.getOwnPropertyDescriptor(Function.prototype, "toString");
				const Page_Proxy = Proxy;
				const Page_WeakMap = WeakMap;
				const Page_WeakMap_get = Page_WeakMap.prototype.get;
				const Page_WeakMap_set = Page_WeakMap.prototype.set;
				// Native function cache - captured before any tampering.
				// A same-origin iframe yields natives uncontaminated by page-level
				// tampering, but at document-start (when this preload runs) there is
				// no documentElement to attach it to. In that case the page itself
				// hasn't executed yet, so window's own natives are still pristine —
				// fall back to window instead of bailing, otherwise none of the
				// fingerprint patches below would ever run.
				let iframe = null;
				const container = document.head ?? document.documentElement;
				if (container) {
					iframe = document.createElement("iframe");
					iframe.style.display = "none";
					container.appendChild(iframe);
					if (!iframe.contentWindow) iframe = null;
				}
				try {
					const nativeWindow = iframe ? iframe.contentWindow : window;

					// Cache pristine native functions
					const Function_toString = nativeWindow.Function.prototype.toString;
					const Object_getOwnPropertyDescriptor = nativeWindow.Object.getOwnPropertyDescriptor;
					const Object_getOwnPropertyDescriptors = nativeWindow.Object.getOwnPropertyDescriptors;
					const Object_getPrototypeOf = nativeWindow.Object.getPrototypeOf;
					const Object_defineProperty = nativeWindow.Object.defineProperty;
					const Object_getOwnPropertyDescriptorOriginal = nativeWindow.Object.getOwnPropertyDescriptor;
					const Object_create = nativeWindow.Object.create;
					const Object_keys = nativeWindow.Object.keys;
					const Object_getOwnPropertyNames = nativeWindow.Object.getOwnPropertyNames;
					const Object_entries = nativeWindow.Object.entries;
					const Object_setPrototypeOf = nativeWindow.Object.setPrototypeOf;
					const Object_assign = nativeWindow.Object.assign;
					const Window_setTimeout = nativeWindow.setTimeout;
					const Math_random = nativeWindow.Math.random;
					const Math_floor = nativeWindow.Math.floor;
					const Math_max = nativeWindow.Math.max;
					const Math_min = nativeWindow.Math.min;
					const Window_Event = nativeWindow.Event;
					const Promise_resolve = nativeWindow.Promise.resolve.bind(nativeWindow.Promise);
					const Window_Blob = nativeWindow.Blob;
					const Window_Proxy = nativeWindow.Proxy;
					const Reflect_get = nativeWindow.Reflect.get;
					const Reflect_set = nativeWindow.Reflect.set;
					const Reflect_apply = nativeWindow.Reflect.apply;
					const Reflect_construct = nativeWindow.Reflect.construct;
					const Reflect_defineProperty = nativeWindow.Reflect.defineProperty;
					const Reflect_deleteProperty = nativeWindow.Reflect.deleteProperty;
					const Reflect_getOwnPropertyDescriptor = nativeWindow.Reflect.getOwnPropertyDescriptor;
					const Reflect_getPrototypeOf = nativeWindow.Reflect.getPrototypeOf;
					const Reflect_has = nativeWindow.Reflect.has;
					const Reflect_isExtensible = nativeWindow.Reflect.isExtensible;
					const Reflect_ownKeys = nativeWindow.Reflect.ownKeys;
					const Reflect_preventExtensions = nativeWindow.Reflect.preventExtensions;
					const Reflect_setPrototypeOf = nativeWindow.Reflect.setPrototypeOf;
					const Intl_DateTimeFormat = nativeWindow.Intl.DateTimeFormat;
					const Date_constructor = nativeWindow.Date;

					const nativeFunctionSources = new Page_WeakMap();
					const makeNativeString = (name) => "function " + (name || "") + "() { [native code] }";
					const registerNativeSource = (fn, source) => {
						if (typeof fn === "function") Reflect_apply(Page_WeakMap_set, nativeFunctionSources, [fn, source]);
						return fn;
					};
					const patchToString = (fn, name) => registerNativeSource(fn, makeNativeString(name));
					if (true) {
						const functionToStringProxy = new Page_Proxy(Page_Function_toString, {
							apply(target, thisArg, args) {
								const source = Reflect_apply(Page_WeakMap_get, nativeFunctionSources, [thisArg]);
								if (source) return source;
								return Reflect_apply(target, thisArg, args || []);
							},
							get(target, key, receiver) {
								return Reflect_get(target, key, receiver);
							},
						});
						registerNativeSource(functionToStringProxy, makeNativeString("toString"));
						Object_defineProperty(Function.prototype, "toString", {
							...(Page_FunctionToStringDescriptor || {
								writable: true,
								configurable: true,
								enumerable: false,
							}),
							value: functionToStringProxy,
						});
					}

					
		try {
			// Register native-looking source for common fingerprinted functions without
// adding own toString properties.
const patchedFns = [
  [window.alert, "alert"],
  [window.prompt, "prompt"],
  [window.confirm, "confirm"],
  [window.fetch, "fetch"],
  [window.XMLHttpRequest, "XMLHttpRequest"],
  [window.WebSocket, "WebSocket"],
  [window.localStorage?.getItem, "getItem"],
  [window.localStorage?.setItem, "setItem"],
  [window.navigator.geolocation?.getCurrentPosition, "getCurrentPosition"],
  [window.navigator.geolocation?.watchPosition, "watchPosition"],
  [window.CanvasRenderingContext2D?.prototype.getImageData, "getImageData"],
  [window.HTMLCanvasElement?.prototype.toDataURL, "toDataURL"],
  [window.HTMLCanvasElement?.prototype.toBlob, "toBlob"],
];

for (const [fn, name] of patchedFns) {
  patchToString(fn, name);
}

// Patch Object.getOwnPropertyDescriptor to return native-looking accessor
// source through the shared Function.prototype.toString registry.
const patchedGetOwnPropertyDescriptor = function getOwnPropertyDescriptor(obj, prop) {
  const descriptor = Reflect_apply(Object_getOwnPropertyDescriptor, this, [obj, prop]);
  if (!descriptor) return descriptor;

  if (descriptor.get && typeof descriptor.get === "function") {
    patchToString(descriptor.get, "get " + String(prop));
  }
  if (descriptor.set && typeof descriptor.set === "function") {
    patchToString(descriptor.set, "set " + String(prop));
  }

  return descriptor;
};
patchToString(patchedGetOwnPropertyDescriptor, "getOwnPropertyDescriptor");
Object_defineProperty(Object, "getOwnPropertyDescriptor", {
  value: patchedGetOwnPropertyDescriptor,
  writable: true,
  configurable: true,
  enumerable: false,
});
;
		} catch (e) {}
	;

		try {
			{
  const visibilityDescriptors = Object_getOwnPropertyDescriptors({
    get hidden() {
      return false;
    },
    get visibilityState() {
      return "visible";
    },
    get webkitHidden() {
      return false;
    },
    get webkitVisibilityState() {
      return "visible";
    },
  });
  for (const key of Object_keys(visibilityDescriptors)) {
    const descriptor = visibilityDescriptors[key];
    if (descriptor && typeof descriptor.get === "function") {
      patchToString(descriptor.get, "get " + key);
    }
  }
  const focusDescriptor = Object_getOwnPropertyDescriptor(
    {
      hasFocus() {
        return document.visibilityState === "visible";
      },
    },
    "hasFocus",
  );
  if (focusDescriptor && typeof focusDescriptor.value === "function") {
    patchToString(focusDescriptor.value, "hasFocus");
  }

  const clearOwnSlot = (name) => {
    const ownDescriptor = Object_getOwnPropertyDescriptor(document, name);
    if (!ownDescriptor) return true;
    if (ownDescriptor.configurable !== true) return false;
    return Reflect_deleteProperty(document, name);
  };

  const inheritedSlot = (name, expectedKind) => {
    let proto = Object_getPrototypeOf(document);
    while (proto) {
      const descriptor = Object_getOwnPropertyDescriptor(proto, name);
      if (descriptor && typeof descriptor[expectedKind] === "function") {
        return [proto, descriptor];
      }
      proto = Object_getPrototypeOf(proto);
    }
  };

  const defineAccessor = (name) => {
    if (!clearOwnSlot(name)) return;
    const slot = inheritedSlot(name, "get");
    if (!slot || slot[1].configurable !== true) return;

    Object_defineProperty(slot[0], name, {
      get: visibilityDescriptors[name].get,
      enumerable: slot[1].enumerable,
      configurable: true,
    });
  };

  defineAccessor("hidden");
  defineAccessor("visibilityState");
  defineAccessor("webkitHidden");
  defineAccessor("webkitVisibilityState");

  if (clearOwnSlot("hasFocus")) {
    const slot = inheritedSlot("hasFocus", "value");
    if (slot && slot[1].configurable === true) {
      Object_defineProperty(slot[0], "hasFocus", {
        value: focusDescriptor.value,
        writable: slot[1].writable === true,
        enumerable: slot[1].enumerable,
        configurable: true,
      });
    }
  }
}
;
		} catch (e) {}
	;

		try {
			{
  const htmlElementOffsetHeightDescriptor =
    typeof HTMLElement === "undefined"
      ? undefined
      : Object_getOwnPropertyDescriptor(HTMLElement.prototype, "offsetHeight");
  const htmlDivElementOffsetHeightDescriptor =
    typeof HTMLDivElement === "undefined"
      ? undefined
      : Object_getOwnPropertyDescriptor(HTMLDivElement.prototype, "offsetHeight");
  const offsetHeightDescriptor =
    htmlDivElementOffsetHeightDescriptor || htmlElementOffsetHeightDescriptor;
  const offsetHeightPrototype = htmlDivElementOffsetHeightDescriptor
    ? HTMLDivElement.prototype
    : htmlElementOffsetHeightDescriptor
      ? HTMLElement.prototype
      : undefined;
  const elementIdDescriptor =
    typeof Element === "undefined"
      ? undefined
      : Object_getOwnPropertyDescriptor(Element.prototype, "id");

  if (
    typeof HTMLDivElement !== "undefined" &&
    offsetHeightPrototype &&
    offsetHeightDescriptor &&
    typeof offsetHeightDescriptor.get === "function" &&
    offsetHeightDescriptor.configurable &&
    elementIdDescriptor &&
    typeof elementIdDescriptor.get === "function"
  ) {
    const offsetHeightGetter = new Window_Proxy(offsetHeightDescriptor.get, {
      apply(target, thisArg, args) {
        const height = Reflect_apply(target, thisArg, args);

        if (
          height === 0 &&
          Object_getPrototypeOf(thisArg) === HTMLDivElement.prototype &&
          Reflect_apply(elementIdDescriptor.get, thisArg, []) === "modernizr"
        ) {
          return 1;
        }

        return height;
      },
    });

    patchToString(offsetHeightGetter, "get offsetHeight");

    Object_defineProperty(
      offsetHeightPrototype,
      "offsetHeight",
      Object_assign({}, offsetHeightDescriptor, {
        get: offsetHeightGetter,
      }),
    );
  }
}
;
		} catch (e) {}
	;

		try {
			
// Ensure navigator.webdriver behaves like real Chrome
if (navigator.webdriver !== false && navigator.webdriver !== undefined) {
	const proto = Object_getPrototypeOf(navigator);
	if (proto && "webdriver" in proto) {
		delete proto.webdriver;
	}
}

// Ensure window.chrome exists and is populated with standard Chrome APIs
if (!window.chrome) {
	Object_defineProperty(window, "chrome", {
		writable: true,
		enumerable: true,
		configurable: false,
		value: {},
	});
}

if (window.chrome && !("app" in window.chrome)) {
	const STATIC_DATA = {
		isInstalled: false,
		InstallState: {
			DISABLED: "disabled",
			INSTALLED: "installed",
			NOT_INSTALLED: "not_installed",
		},
		RunningState: {
			CANNOT_RUN: "cannot_run",
			READY_TO_RUN: "ready_to_run",
			RUNNING: "running",
		},
	};

	const makeError = (fn) => new TypeError(`Error in invocation of app.${fn}()`);

	window.chrome.app = {
		...STATIC_DATA,
		get isInstalled() {
			return false;
		},
		getDetails: function getDetails() {
			if (arguments.length) throw makeError("getDetails");
			return null;
		},
		getIsInstalled: function getIsInstalled() {
			if (arguments.length) throw makeError("getIsInstalled");
			return false;
		},
		runningState: function runningState() {
			if (arguments.length) throw makeError("runningState");
			return "cannot_run";
		},
	};

	patchToString(window.chrome.app.getDetails, "getDetails");
	patchToString(window.chrome.app.getIsInstalled, "getIsInstalled");
	patchToString(window.chrome.app.runningState, "runningState");
}

if (window.chrome && !("csi" in window.chrome) && window.performance?.timing) {
	window.chrome.csi = function csi() {
		const { timing } = window.performance;
		return {
			onloadT: timing.domContentLoadedEventEnd,
			startE: timing.navigationStart,
			pageT: Date.now() - timing.navigationStart,
			tran: 15,
		};
	};
	patchToString(window.chrome.csi, "csi");
}

if (
	window.chrome &&
	!("loadTimes" in window.chrome) &&
	window.performance?.timing &&
	window.PerformancePaintTiming
) {
	const { performance } = window;
	const ntEntryFallback = {
		nextHopProtocol: "h2",
		type: "other",
	};

	const protocolInfo = {
		get connectionInfo() {
			const ntEntry = performance.getEntriesByType("navigation")[0] || ntEntryFallback;
			return ntEntry.nextHopProtocol;
		},
		get npnNegotiatedProtocol() {
			const ntEntry = performance.getEntriesByType("navigation")[0] || ntEntryFallback;
			return ["h2", "hq"].includes(ntEntry.nextHopProtocol) ? ntEntry.nextHopProtocol : "unknown";
		},
		get navigationType() {
			const ntEntry = performance.getEntriesByType("navigation")[0] || ntEntryFallback;
			return ntEntry.type;
		},
		get wasAlternateProtocolAvailable() {
			return false;
		},
		get wasFetchedViaSpdy() {
			const ntEntry = performance.getEntriesByType("navigation")[0] || ntEntryFallback;
			return ["h2", "hq"].includes(ntEntry.nextHopProtocol);
		},
		get wasNpnNegotiated() {
			const ntEntry = performance.getEntriesByType("navigation")[0] || ntEntryFallback;
			return ["h2", "hq"].includes(ntEntry.nextHopProtocol);
		},
	};

	const { timing } = window.performance;

	const toFixed = (num, fixed) => {
		const re = new RegExp("^-?\\d+(?:.\\d{0," + (fixed || -1) + "})?");
		return num.toString().match(re)[0];
	};

	const timingInfo = {
		get firstPaintAfterLoadTime() {
			return 0;
		},
		get requestTime() {
			return timing.navigationStart / 1000;
		},
		get startLoadTime() {
			return timing.navigationStart / 1000;
		},
		get commitLoadTime() {
			return timing.responseStart / 1000;
		},
		get finishDocumentLoadTime() {
			return timing.domContentLoadedEventEnd / 1000;
		},
		get finishLoadTime() {
			return timing.loadEventEnd / 1000;
		},
		get firstPaintTime() {
			const fpEntry = performance.getEntriesByType("paint")[0] || {
				startTime: timing.loadEventEnd / 1000,
			};
			return toFixed((fpEntry.startTime + performance.timeOrigin) / 1000, 3);
		},
	};

	window.chrome.loadTimes = function loadTimes() {
		return { ...protocolInfo, ...timingInfo };
	};
	patchToString(window.chrome.loadTimes, "loadTimes");
}

const isSecureOrigin = document.location.protocol.startsWith("https");
if (window.chrome && !("runtime" in window.chrome) && isSecureOrigin) {
	const STATIC_DATA = {
		OnInstalledReason: {
			CHROME_UPDATE: "chrome_update",
			INSTALL: "install",
			SHARED_MODULE_UPDATE: "shared_module_update",
			UPDATE: "update",
		},
		OnRestartRequiredReason: {
			APP_UPDATE: "app_update",
			OS_UPDATE: "os_update",
			PERIODIC: "periodic",
		},
		PlatformArch: {
			ARM: "arm",
			ARM64: "arm64",
			MIPS: "mips",
			MIPS64: "mips64",
			X86_32: "x86-32",
			X86_64: "x86-64",
		},
		PlatformNaclArch: {
			ARM: "arm",
			MIPS: "mips",
			MIPS64: "mips64",
			X86_32: "x86-32",
			X86_64: "x86-64",
		},
		PlatformOs: {
			ANDROID: "android",
			CROS: "cros",
			LINUX: "linux",
			MAC: "mac",
			OPENBSD: "openbsd",
			WIN: "win",
		},
		RequestUpdateCheckStatus: {
			NO_UPDATE: "no_update",
			THROTTLED: "throttled",
			UPDATE_AVAILABLE: "update_available",
		},
	};

	const makeCustomRuntimeErrors = (preamble, method, extensionId) => ({
		NoMatchingSignature: new TypeError(preamble + "No matching signature."),
		MustSpecifyExtensionID: new TypeError(
			preamble +
				`${method} called from a webpage must specify an Extension ID (string) for its first argument.`,
		),
		InvalidExtensionID: new TypeError(preamble + `Invalid extension id: '${extensionId}'`),
	});

	const isValidExtensionId = (value) => value.length === 32 && value.toLowerCase().match(/^[a-p]+$/);

	const sendMessageHandler = {
		apply(target, _ctx, args) {
			const [extensionId, options, responseCallback] = args || [];
			const errorPreamble =
				"Error in invocation of runtime.sendMessage(optional string extensionId, any message, optional object options, optional function responseCallback): ";
			const Errors = makeCustomRuntimeErrors(errorPreamble, "chrome.runtime.sendMessage()", extensionId);

			const noArguments = args.length === 0;
			const tooManyArguments = args.length > 4;
			const incorrectOptions = options && typeof options !== "object";
			const incorrectResponseCallback = responseCallback && typeof responseCallback !== "function";

			if (noArguments || tooManyArguments || incorrectOptions || incorrectResponseCallback) {
				throw Errors.NoMatchingSignature;
			}

			if (args.length < 2) {
				throw Errors.MustSpecifyExtensionID;
			}

			if (typeof extensionId !== "string") {
				throw Errors.NoMatchingSignature;
			}

			if (!isValidExtensionId(extensionId)) {
				throw Errors.InvalidExtensionID;
			}

			return undefined;
		},
	};

	const connectHandler = {
		apply(target, _ctx, args) {
			const [extensionId, connectInfo] = args || [];
			const errorPreamble =
				"Error in invocation of runtime.connect(optional string extensionId, optional object connectInfo): ";
			const Errors = makeCustomRuntimeErrors(errorPreamble, "chrome.runtime.connect()", extensionId);

			const noArguments = args.length === 0;
			const emptyStringArgument = args.length === 1 && extensionId === "";
			if (noArguments || emptyStringArgument) {
				throw Errors.MustSpecifyExtensionID;
			}

			const tooManyArguments = args.length > 2;
			const incorrectConnectInfoType = connectInfo && typeof connectInfo !== "object";
			if (tooManyArguments || incorrectConnectInfoType) {
				throw Errors.NoMatchingSignature;
			}

			const extensionIdIsString = typeof extensionId === "string";
			if (extensionIdIsString && extensionId === "") {
				throw Errors.MustSpecifyExtensionID;
			}
			if (extensionIdIsString && !isValidExtensionId(extensionId)) {
				throw Errors.InvalidExtensionID;
			}

			const validateConnectInfo = (info) => {
				if (args.length > 1) {
					throw Errors.NoMatchingSignature;
				}
				if (Object_keys(info).length === 0) {
					throw Errors.MustSpecifyExtensionID;
				}
				Object_entries(info).forEach(([key, value]) => {
					const isExpected = ["name", "includeTlsChannelId"].includes(key);
					if (!isExpected) {
						throw new TypeError(errorPreamble + `Unexpected property: '${key}'.`);
					}
					const mismatch = (propName, expected, found) =>
						TypeError(
							errorPreamble +
								`Error at property '${propName}': Invalid type: expected ${expected}, found ${found}.`,
						);
					if (key === "name" && typeof value !== "string") {
						throw mismatch(key, "string", typeof value);
					}
					if (key === "includeTlsChannelId" && typeof value !== "boolean") {
						throw mismatch(key, "boolean", typeof value);
					}
				});
			};

			if (typeof extensionId === "object") {
				validateConnectInfo(extensionId);
				throw Errors.MustSpecifyExtensionID;
			}

			const makeConnectResponse = () => {
				const onSomething = () => ({
					addListener: function addListener() {},
					dispatch: function dispatch() {},
					hasListener: function hasListener() {},
					hasListeners: function hasListeners() {
						return false;
					},
					removeListener: function removeListener() {},
				});

				return {
					name: "",
					sender: undefined,
					disconnect: function disconnect() {},
					onDisconnect: onSomething(),
					onMessage: onSomething(),
					postMessage: function postMessage() {
						if (!arguments.length) {
							throw new TypeError("Insufficient number of arguments.");
						}
						throw new Error("Attempting to use a disconnected port object");
					},
				};
			};

			return makeConnectResponse();
		},
	};

	window.chrome.runtime = {
		...STATIC_DATA,
		get id() {
			return undefined;
		},
		connect: null,
		sendMessage: null,
	};

	const sendMessageProxy = new Window_Proxy(function sendMessage() {}, sendMessageHandler);
	const connectProxy = new Window_Proxy(function connect() {}, connectHandler);
	patchToString(sendMessageProxy, "sendMessage");
	patchToString(connectProxy, "connect");
	window.chrome.runtime.sendMessage = sendMessageProxy;
	window.chrome.runtime.connect = connectProxy;
}

// Suppress Permission.query for automation-controlled
if (navigator.permissions?.query) {
	if (isSecureOrigin && "Notification" in window) {
		const notificationPermissionGetter = Object_getOwnPropertyDescriptor({
			get permission() {
				return "default";
			},
		}, "permission").get;
		patchToString(notificationPermissionGetter, "get permission");
		Object_defineProperty(Notification, "permission", {
			get: notificationPermissionGetter,
			configurable: true,
		});
	} else if (!isSecureOrigin) {
		const originalQuery = navigator.permissions.query;
		const patchedPermissionsQuery = function query(parameters) {
			if (parameters?.name === "notifications") {
			const status = { state: "denied", onchange: null };
			if (typeof PermissionStatus !== "undefined") {
				Object_setPrototypeOf(status, PermissionStatus.prototype);
			}
			return Promise_resolve(status);
			}
			return originalQuery.call(this, parameters);
		};
		patchToString(patchedPermissionsQuery, "query");
		navigator.permissions.query = patchedPermissionsQuery;
	}
}

// Remove CDC_ markers from document
const documentProps = Object_getOwnPropertyNames(document);
for (const prop of documentProps) {
	if (prop.startsWith("cdc_")) {
		delete document[prop];
	}
}
;
		} catch (e) {}
	;

		try {
			const iframeWindowProxies = new WeakMap();

const isFrameIndexKey = (key) => {
	if (typeof key !== "string" || key === "") return false;
	const number = +key;
	return number >= 0 && number < 4294967295 && Math_floor(number) === number && `${number}` === key;
};

const getPropertyDescriptor = (object, key) => {
	let current = object;
	while (current) {
		const descriptor = Object_getOwnPropertyDescriptor(current, key);
		if (descriptor) return descriptor;
		current = Object_getPrototypeOf(current);
	}
};

const descriptorWithValue = (target, key, value, writable) => {
	const descriptor = Reflect_getOwnPropertyDescriptor(target, key);
	if (descriptor && descriptor.configurable === false) return descriptor;
	const next = descriptor ? Object_assign({}, descriptor) : { configurable: true, enumerable: true };
	Reflect_deleteProperty(next, "get");
	Reflect_deleteProperty(next, "set");
	next.value = value;
	if (!Reflect_has(next, "writable")) next.writable = writable;
	return next;
};
const iframeContentWindowDescriptor =
	typeof HTMLIFrameElement === "undefined"
		? undefined
		: getPropertyDescriptor(HTMLIFrameElement.prototype, "contentWindow");
const iframeSrcdocDescriptor =
	typeof HTMLIFrameElement === "undefined"
		? undefined
		: getPropertyDescriptor(HTMLIFrameElement.prototype, "srcdoc");

const getNativeContentWindow = (iframe) => {
	if (iframeContentWindowDescriptor && iframeContentWindowDescriptor.get) {
		return Reflect_apply(iframeContentWindowDescriptor.get, iframe, []);
	}
	return undefined;
};

const addContentWindowProxy = (iframe) => {
	let state = Reflect_apply(Page_WeakMap_get, iframeWindowProxies, [iframe]);
	if (state) return state.proxy;

	state = { proxy: undefined, target: undefined, wasConnected: false, setProxyTarget: undefined };

	const isDiscarded = () => {
		if (iframe.isConnected) state.wasConnected = true;
		return state.wasConnected && !iframe.isConnected;
	};

	const currentFrameElement = () => {
		if (iframe.isConnected) {
			state.wasConnected = true;
			return iframe;
		}
		return state.wasConnected ? null : iframe;
	};

	const contentWindowProxy = {
		get(target, key) {
			if (key === "self" || key === "window" || key === "frames" || key === "globalThis") return state.proxy;
			if (key === "frameElement") return currentFrameElement();
			if (key === "closed" && isDiscarded()) return true;
			if (isFrameIndexKey(key)) return undefined;
			if (key === "length") return 0;
			return Reflect_get(target, key);
		},
		getOwnPropertyDescriptor(target, key) {
			if (key === "self" || key === "window" || key === "frames" || key === "globalThis") {
				return descriptorWithValue(target, key, state.proxy, true);
			}
			if (key === "frameElement") {
				return descriptorWithValue(target, key, currentFrameElement(), false);
			}
			if (key === "closed" && isDiscarded()) {
				return descriptorWithValue(target, key, true, false);
			}
			if (isFrameIndexKey(key)) return undefined;
			if (key === "length") return descriptorWithValue(target, key, 0, false);
			return Reflect_getOwnPropertyDescriptor(target, key);
		},
		has(target, key) {
			if (key === "self" || key === "window" || key === "frames" || key === "globalThis" || key === "frameElement") return true;
			if (isFrameIndexKey(key)) return false;
			return Reflect_has(target, key);
		},
		ownKeys(target) {
			const keys = Reflect_ownKeys(target);
			const filtered = [];
			for (let index = 0; index < keys.length; index++) {
				if (!isFrameIndexKey(keys[index])) filtered.push(keys[index]);
			}
			return filtered;
		},
	};

	const setProxyTarget = (target) => {
		if (state.target === target && state.proxy) return state.proxy;
		state.target = target;
		state.proxy = new Window_Proxy(target, contentWindowProxy);
		return state.proxy;
	};
	state.setProxyTarget = setProxyTarget;
	const initialContentWindow = getNativeContentWindow(iframe);
	if (initialContentWindow) setProxyTarget(initialContentWindow);
	if (iframe.isConnected) state.wasConnected = true;
	Reflect_apply(Page_WeakMap_set, iframeWindowProxies, [iframe, state]);


	return state.proxy;
};
if (
	iframeContentWindowDescriptor &&
	iframeContentWindowDescriptor.get &&
	iframeContentWindowDescriptor.configurable !== false
) {
	const contentWindowAccessors = {
		get contentWindow() {
			const state = Reflect_apply(Page_WeakMap_get, iframeWindowProxies, [this]);
			if (!state) return getNativeContentWindow(this);
			if (this.isConnected) state.wasConnected = true;
			if (state.wasConnected && !this.isConnected) return null;
			const nativeContentWindow = getNativeContentWindow(this);
			if (!nativeContentWindow) return nativeContentWindow;
			return state.setProxyTarget(nativeContentWindow);
		},
	};
	const contentWindowGetter = Object_getOwnPropertyDescriptor(contentWindowAccessors, "contentWindow").get;
	patchToString(contentWindowGetter, "get contentWindow");
	Object_defineProperty(HTMLIFrameElement.prototype, "contentWindow", {
		get: contentWindowGetter,
		set: iframeContentWindowDescriptor.set,
		enumerable: iframeContentWindowDescriptor.enumerable,
		configurable: iframeContentWindowDescriptor.configurable,
	});
}


if (
	iframeSrcdocDescriptor &&
	iframeSrcdocDescriptor.configurable !== false
) {
	const srcdocAccessors = {
		get srcdoc() {
			if (iframeSrcdocDescriptor.get) {
				return Reflect_apply(iframeSrcdocDescriptor.get, this, []);
			}
			const value = this.getAttribute("srcdoc");
			return value === null ? "" : value;
		},
		set srcdoc(newValue) {
			addContentWindowProxy(this);
			if (iframeSrcdocDescriptor.set) {
				return Reflect_apply(iframeSrcdocDescriptor.set, this, [newValue]);
			}
			return this.setAttribute("srcdoc", newValue);
		},
	};
	const srcdocDescriptor = Object_getOwnPropertyDescriptor(srcdocAccessors, "srcdoc");
	const srcdocGetter = srcdocDescriptor.get;
	const srcdocSetter = srcdocDescriptor.set;
	patchToString(srcdocGetter, "get srcdoc");
	patchToString(srcdocSetter, "set srcdoc");
	Object_defineProperty(HTMLIFrameElement.prototype, "srcdoc", {
		get: srcdocGetter,
		set: srcdocSetter,
		enumerable: iframeSrcdocDescriptor.enumerable,
		configurable: iframeSrcdocDescriptor.configurable,
	});
}
;
		} catch (e) {}
	;

		try {
			const webglPlatformSource =
	String(navigator.userAgentData?.platform || navigator.platform || "") +
	" " +
	String(navigator.userAgent || "");
const webglPlatformText = webglPlatformSource.toLowerCase();
const webglPlatformKind = webglPlatformText.includes("android")
	? "android"
	: webglPlatformText.includes("iphone") ||
		  webglPlatformText.includes("ipad") ||
		  webglPlatformText.includes("ipod")
		? "ios"
		: webglPlatformText.includes("mac")
			? "mac"
			: webglPlatformText.includes("win")
				? "windows"
				: webglPlatformText.includes("cros")
					? "cros"
					: "linux";

const webglFallbackProfiles = {
	android: {
		vendor: "Qualcomm",
		renderer: "Adreno (TM) 640",
	},
	ios: {
		vendor: "Apple Inc.",
		renderer: "Apple GPU",
	},
	mac: {
		vendor: "Google Inc. (Intel Inc.)",
		renderer: "ANGLE (Intel Inc., Intel(R) Iris(TM) Plus Graphics 640 OpenGL Engine, OpenGL 4.1)",
	},
	windows: {
		vendor: "Google Inc. (Intel)",
		renderer: "ANGLE (Intel, Intel(R) UHD Graphics 620 Direct3D11 vs_5_0 ps_5_0, D3D11)",
	},
	cros: {
		vendor: "Google Inc. (Intel)",
		renderer: "ANGLE (Intel, Mesa Intel(R) UHD Graphics 620 (KBL GT2), OpenGL 4.6)",
	},
	linux: {
		vendor: "Google Inc. (Intel)",
		renderer: "ANGLE (Intel, Mesa Intel(R) UHD Graphics 620 (KBL GT2), OpenGL 4.6)",
	},
};
const webglFallbackProfile = webglFallbackProfiles[webglPlatformKind] || webglFallbackProfiles.linux;
const webglContextProfiles = new WeakMap();
const webglIsObjectKey = (value) =>
	(typeof value === "object" && value !== null) || typeof value === "function";

const webglLooksSoftware = (value) => {
	const text = String(value || "").toLowerCase();
	return (
		text.includes("swiftshader") ||
		text.includes("llvmpipe") ||
		text.includes("lavapipe") ||
		text.includes("software") ||
		text.includes("mesa offscreen") ||
		text.includes("google inc. (google)")
	);
};

const webglMatchesPlatform = (renderer) => {
	const text = String(renderer || "").toLowerCase();
	if (webglPlatformKind === "windows") {
		return !text.includes("apple") && !text.includes("mesa") && !text.includes("opengl engine");
	}
	if (webglPlatformKind === "mac") {
		return !text.includes("direct3d") && !text.includes("d3d") && !text.includes("mesa");
	}
	if (webglPlatformKind === "android") {
		return (
			text.includes("adreno") ||
			text.includes("mali") ||
			text.includes("powervr") ||
			text.includes("qualcomm")
		);
	}
	if (webglPlatformKind === "ios") {
		return text.includes("apple");
	}
	return !text.includes("direct3d") && !text.includes("d3d") && !text.includes("apple");
};

const webglGetContextProfile = (target, thisArg) => {
	if (!webglIsObjectKey(thisArg)) return webglFallbackProfile;
	const cached = Reflect_apply(Page_WeakMap_get, webglContextProfiles, [thisArg]);
	if (cached) return cached;

	let nativeVendor;
	let nativeRenderer;
	try {
		nativeVendor = Reflect_apply(target, thisArg, [0x9245]);
		nativeRenderer = Reflect_apply(target, thisArg, [0x9246]);
	} catch {}

	const nativeProfile =
		typeof nativeVendor === "string" &&
		typeof nativeRenderer === "string" &&
		nativeVendor &&
		nativeRenderer &&
		!webglLooksSoftware(nativeVendor) &&
		!webglLooksSoftware(nativeRenderer) &&
		webglMatchesPlatform(nativeRenderer)
			? { vendor: nativeVendor, renderer: nativeRenderer }
			: webglFallbackProfile;

	Reflect_apply(Page_WeakMap_set, webglContextProfiles, [thisArg, nativeProfile]);
	return nativeProfile;
};

const webglGetParameterHandler = {
	apply(target, thisArg, args) {
		const nativeValue = Reflect_apply(target, thisArg, args);
		const param = args[0];
		if (param === 0x1f00 && typeof nativeValue === "string") return "WebKit";
		if (param === 0x1f01 && typeof nativeValue === "string") return "WebKit WebGL";
		if ((param === 0x9245 || param === 0x9246) && typeof nativeValue === "string") {
			const profile = webglGetContextProfile(target, thisArg);
			return param === 0x9245 ? profile.vendor : profile.renderer;
		}
		return nativeValue;
	},
};

const webglFloatPrecisionTypes = {
	0x8df0: true,
	0x8df1: true,
	0x8df2: true,
};

const webglClonePrecisionFormat = (result, values) => {
	const ownKeys = Reflect_ownKeys(result);
	let hasRangeMin = false;
	let hasRangeMax = false;
	let hasPrecision = false;
	for (let index = 0; index < ownKeys.length; index += 1) {
		if (ownKeys[index] === "rangeMin") hasRangeMin = true;
		if (ownKeys[index] === "rangeMax") hasRangeMax = true;
		if (ownKeys[index] === "precision") hasPrecision = true;
	}
	if (!hasRangeMin || !hasRangeMax || !hasPrecision) return result;

	const clone = Object_create(Object_getPrototypeOf(result));
	for (let index = 0; index < ownKeys.length; index += 1) {
		const key = ownKeys[index];
		const descriptor = Object_getOwnPropertyDescriptor(result, key);
		if (!descriptor) return result;
		if (key === "rangeMin" || key === "rangeMax" || key === "precision") {
			if (!("value" in descriptor)) return result;
			descriptor.value = values[key];
		}
		try {
			Object_defineProperty(clone, key, descriptor);
		} catch {
			return result;
		}
	}
	return clone;
};

const webglGetShaderPrecisionFormatHandler = {
	apply(target, thisArg, args) {
		const result = Reflect_apply(target, thisArg, args);
		const precisionType = args[1];
		if (
			!result ||
			webglPlatformKind === "android" ||
			webglPlatformKind === "ios" ||
			!webglFloatPrecisionTypes[precisionType]
		) {
			return result;
		}

		const rangeMin = result.rangeMin;
		const rangeMax = result.rangeMax;
		const precision = result.precision;
		if (
			typeof rangeMin !== "number" ||
			typeof rangeMax !== "number" ||
			typeof precision !== "number"
		) {
			return result;
		}

		const values = {
			rangeMin: Math_max(rangeMin, 127),
			rangeMax: Math_max(rangeMax, 127),
			precision: Math_max(precision, 23),
		};
		if (
			values.rangeMin === rangeMin &&
			values.rangeMax === rangeMax &&
			values.precision === precision
		) {
			return result;
		}
		return webglClonePrecisionFormat(result, values);
	},
};

const webglInstallMethodProxy = (proto, name, handler) => {
	if (!proto) return;
	const descriptor = Object_getOwnPropertyDescriptor(proto, name);
	if (!descriptor || typeof descriptor.value !== "function") return;
	const proxy = new Window_Proxy(descriptor.value, handler);
	patchToString(proxy, name);
	try {
		Object_defineProperty(proto, name, {
			value: proxy,
			writable: descriptor.writable,
			configurable: descriptor.configurable,
			enumerable: descriptor.enumerable,
		});
	} catch {}
};

if (window.WebGLRenderingContext) {
	webglInstallMethodProxy(WebGLRenderingContext.prototype, "getParameter", webglGetParameterHandler);
	webglInstallMethodProxy(
		WebGLRenderingContext.prototype,
		"getShaderPrecisionFormat",
		webglGetShaderPrecisionFormatHandler,
	);
}
if (window.WebGL2RenderingContext) {
	webglInstallMethodProxy(WebGL2RenderingContext.prototype, "getParameter", webglGetParameterHandler);
	webglInstallMethodProxy(
		WebGL2RenderingContext.prototype,
		"getShaderPrecisionFormat",
		webglGetShaderPrecisionFormatHandler,
	);
}
;
		} catch (e) {}
	;

		try {
			;(() => {
  if (typeof Window_Proxy !== "function" || typeof Reflect_apply !== "function") return;

  const screenObject = window.screen;
  if (!screenObject) return;

  const isFiniteNumber = (value) =>
    typeof value === "number" &&
    value === value &&
    value !== Infinity &&
    value !== -Infinity;

  const readValue = (object, prop, fallback) => {
    try {
      const value = object[prop];
      return value === undefined ? fallback : value;
    } catch {
      return fallback;
    }
  };

  const finiteNumber = (value, fallback) =>
    isFiniteNumber(value) ? value : fallback;

  const positiveNumber = (value, fallback) =>
    isFiniteNumber(value) && value > 0 ? value : fallback;

  const integerNumber = (value, fallback) =>
    isFiniteNumber(value) ? Math_floor(value) : fallback;

  const positiveInteger = (value, fallback) =>
    isFiniteNumber(value) && value > 0 ? Math_floor(value) : fallback;

  const findDescriptorOwner = (object, prop) => {
    let owner = object;
    while (owner) {
      let descriptor;
      try {
        descriptor = Object_getOwnPropertyDescriptor(owner, prop);
      } catch {
        return undefined;
      }
      if (descriptor) return [owner, descriptor];
      try {
        owner = Object_getPrototypeOf(owner);
      } catch {
        return undefined;
      }
    }
    return undefined;
  };

  const patchGetter = (object, prop, getValue) => {
    const found = findDescriptorOwner(object, prop);
    if (!found) return false;

    const owner = found[0];
    const descriptor = found[1];
    if (descriptor.configurable !== true || typeof descriptor.get !== "function") {
      return false;
    }

    const originalGet = descriptor.get;
    const patchedGet = new Window_Proxy(originalGet, {
      apply(target, thisArg, args) {
        const nativeValue = Reflect_apply(target, thisArg, args);
        return getValue(nativeValue, thisArg);
      },
    });
    patchToString(patchedGet, "get " + prop);
    try {
      Object_defineProperty(owner, prop, {
        configurable: descriptor.configurable,
        enumerable: descriptor.enumerable,
        get: patchedGet,
        set: descriptor.set,
      });
      return true;
    } catch {
      return false;
    }
  };

  const patchStableNumber = (object, prop, value) => {
    const current = readValue(object, prop, undefined);
    if (current === value) return;
    patchGetter(object, prop, () => value);
  };

  const initialInnerWidth = positiveInteger(readValue(window, "innerWidth", 0), 0);
  const initialInnerHeight = positiveInteger(readValue(window, "innerHeight", 0), 0);
  const initialOuterWidth = positiveInteger(
    readValue(window, "outerWidth", Math_max(initialInnerWidth, 1)),
    Math_max(initialInnerWidth, 1),
  );
  const initialOuterHeight = positiveInteger(
    readValue(window, "outerHeight", Math_max(initialInnerHeight, 1)),
    Math_max(initialInnerHeight, 1),
  );
  // Real Chrome reserves vertical space for the tab strip + URL bar, so
  // outerHeight is always taller than innerHeight (~88px on a stock window).
  // Headless reports outerHeight === innerHeight, which is a well-known tell, so
  // synthesize a realistic chrome height when the window has no visible chrome.
  const browserChromeHeight = 88;
  const targetOuterHeight =
    initialOuterHeight > initialInnerHeight
      ? initialOuterHeight
      : initialInnerHeight + browserChromeHeight;
  const requiredWidth = Math_max(initialInnerWidth, initialOuterWidth, 1);
  const requiredHeight = Math_max(initialInnerHeight, initialOuterHeight, targetOuterHeight, 1);

  const screenWidth = Math_max(
    positiveInteger(readValue(screenObject, "width", requiredWidth), requiredWidth),
    requiredWidth,
  );
  const screenHeight = Math_max(
    positiveInteger(readValue(screenObject, "height", requiredHeight), requiredHeight),
    requiredHeight,
  );

  let screenAvailWidth = positiveInteger(
    readValue(screenObject, "availWidth", screenWidth),
    screenWidth,
  );
  if (screenAvailWidth < requiredWidth || screenAvailWidth > screenWidth) {
    screenAvailWidth = screenWidth;
  }

  let screenAvailHeight = positiveInteger(
    readValue(screenObject, "availHeight", screenHeight),
    screenHeight,
  );
  if (screenAvailHeight < requiredHeight || screenAvailHeight > screenHeight) {
    screenAvailHeight = screenHeight;
  }

  const screenAvailLeft = integerNumber(readValue(screenObject, "availLeft", 0), 0);
  const screenAvailTop = integerNumber(readValue(screenObject, "availTop", 0), 0);
  const screenColorDepth = positiveInteger(readValue(screenObject, "colorDepth", 24), 24);
  const screenPixelDepth = positiveInteger(
    readValue(screenObject, "pixelDepth", screenColorDepth),
    screenColorDepth,
  );

  for (const entry of [
    ["width", screenWidth],
    ["height", screenHeight],
    ["availWidth", screenAvailWidth],
    ["availHeight", screenAvailHeight],
    ["availLeft", screenAvailLeft],
    ["availTop", screenAvailTop],
    ["colorDepth", screenColorDepth],
    ["pixelDepth", screenPixelDepth],
  ]) {
    patchStableNumber(screenObject, entry[0], entry[1]);
  }

  const outerWidthNeedsPatch =
    !isFiniteNumber(readValue(window, "outerWidth", undefined)) ||
    readValue(window, "outerWidth", 0) <= 0 ||
    Math_floor(readValue(window, "outerWidth", 0)) < initialInnerWidth;
  if (outerWidthNeedsPatch) {
    patchGetter(window, "outerWidth", (nativeValue) =>
      Math_max(
        positiveInteger(nativeValue, initialOuterWidth),
        positiveInteger(readValue(window, "innerWidth", requiredWidth), requiredWidth),
      ),
    );
  }

  const outerHeightNeedsPatch =
    !isFiniteNumber(readValue(window, "outerHeight", undefined)) ||
    readValue(window, "outerHeight", 0) <= 0 ||
    Math_floor(readValue(window, "outerHeight", 0)) <= initialInnerHeight;
  if (outerHeightNeedsPatch) {
    patchGetter(window, "outerHeight", (nativeValue) =>
      Math_max(
        positiveInteger(nativeValue, initialOuterHeight),
        positiveInteger(readValue(window, "innerHeight", requiredHeight), requiredHeight) +
          browserChromeHeight,
      ),
    );
  }

  const initialDevicePixelRatio = positiveNumber(
    readValue(window, "devicePixelRatio", 1),
    1,
  );
  if (readValue(window, "devicePixelRatio", undefined) !== initialDevicePixelRatio) {
    patchGetter(window, "devicePixelRatio", () => initialDevicePixelRatio);
  }

  const visualViewportObject = readValue(window, "visualViewport", null);
  if (visualViewportObject) {
    const initialScale = positiveNumber(readValue(visualViewportObject, "scale", 1), 1);
    if (readValue(visualViewportObject, "scale", undefined) !== initialScale) {
      patchGetter(visualViewportObject, "scale", (nativeValue) =>
        positiveNumber(nativeValue, initialScale),
      );
    }

    const getViewportScale = () =>
      positiveNumber(readValue(visualViewportObject, "scale", initialScale), initialScale);

    const getViewportWidthFallback = () =>
      positiveNumber(readValue(window, "innerWidth", requiredWidth), requiredWidth) /
      getViewportScale();

    const getViewportHeightFallback = () =>
      positiveNumber(readValue(window, "innerHeight", requiredHeight), requiredHeight) /
      getViewportScale();

    if (positiveNumber(readValue(visualViewportObject, "width", 0), 0) <= 0) {
      patchGetter(visualViewportObject, "width", (nativeValue) =>
        positiveNumber(nativeValue, getViewportWidthFallback()),
      );
    }

    if (positiveNumber(readValue(visualViewportObject, "height", 0), 0) <= 0) {
      patchGetter(visualViewportObject, "height", (nativeValue) =>
        positiveNumber(nativeValue, getViewportHeightFallback()),
      );
    }

    const getViewportOffsetLeft = () =>
      finiteNumber(readValue(visualViewportObject, "offsetLeft", 0), 0);
    const getViewportOffsetTop = () =>
      finiteNumber(readValue(visualViewportObject, "offsetTop", 0), 0);

    if (!isFiniteNumber(readValue(visualViewportObject, "offsetLeft", undefined))) {
      patchGetter(visualViewportObject, "offsetLeft", (nativeValue) =>
        finiteNumber(nativeValue, 0),
      );
    }

    if (!isFiniteNumber(readValue(visualViewportObject, "offsetTop", undefined))) {
      patchGetter(visualViewportObject, "offsetTop", (nativeValue) =>
        finiteNumber(nativeValue, 0),
      );
    }

    if (!isFiniteNumber(readValue(visualViewportObject, "pageLeft", undefined))) {
      patchGetter(visualViewportObject, "pageLeft", (nativeValue) =>
        finiteNumber(
          nativeValue,
          finiteNumber(readValue(window, "scrollX", 0), 0) + getViewportOffsetLeft(),
        ),
      );
    }

    if (!isFiniteNumber(readValue(visualViewportObject, "pageTop", undefined))) {
      patchGetter(visualViewportObject, "pageTop", (nativeValue) =>
        finiteNumber(
          nativeValue,
          finiteNumber(readValue(window, "scrollY", 0), 0) + getViewportOffsetTop(),
        ),
      );
    }
  }
})();
;
		} catch (e) {}
	;

		try {
			const commonFonts = [
  "Arial",
  "Arial Black",
  "Arial Narrow",
  "Arial Rounded MT Bold",
  "Book Antiqua",
  "Bookman Old Style",
  "Calibri",
  "Cambria",
  "Cambria Math",
  "Century",
  "Century Gothic",
  "Century Schoolbook",
  "Comic Sans MS",
  "Consolas",
  "Courier",
  "Courier New",
  "Garamond",
  "Geneva",
  "Georgia",
  "Gill Sans",
  "Gill Sans MT",
  "Helvetica",
  "Helvetica Neue",
  "Impact",
  "Lucida Console",
  "Lucida Grande",
  "Lucida Sans Unicode",
  "MS Gothic",
  "MS PGothic",
  "MS Sans Serif",
  "MS Serif",
  "Palatino",
  "Palatino Linotype",
  "Segoe Print",
  "Segoe Script",
  "Segoe UI",
  "Segoe UI Emoji",
  "Segoe UI Symbol",
  "Tahoma",
  "Times",
  "Times New Roman",
  "Trebuchet MS",
  "Verdana",
  "Wingdings",
  "Wingdings 2",
  "Wingdings 3",
  "Apple Color Emoji",
  "Apple SD Gothic Neo",
  "Hoefler Text",
  "Menlo",
  "Monaco",
  "San Francisco",
  "SF Pro Display",
  "SF Pro Text",
];

if ("queryLocalFonts" in window) {
  const queryLocalFonts = async function queryLocalFonts() {
    return commonFonts.map((family) => ({
      family,
      fullName: family,
      postscriptName: family.replace(/\s+/g, ""),
      style: "Regular",
      blob: () => Promise_resolve(new Window_Blob([])),
    }));
  };
  patchToString(queryLocalFonts, "queryLocalFonts");
  Object_defineProperty(window, "queryLocalFonts", {
    value: queryLocalFonts,
    writable: true,
    configurable: true,
    enumerable: true,
  });
}

const originalGetContext = HTMLCanvasElement.prototype.getContext;
const patchedGetContext = function getContext(type, options) {
  const ctx = originalGetContext.call(this, type, options);
  if (ctx && type === "2d") {
    const originalFillText = ctx.fillText;
    const patchedFillText = function fillText(text, x, y, maxWidth) {
      const noiseX = (Math_random() - 0.5) * 0.02;
      const noiseY = (Math_random() - 0.5) * 0.02;
      return originalFillText.call(
        this,
        text,
        x + noiseX,
        y + noiseY,
        maxWidth,
      );
    };
    patchToString(patchedFillText, "fillText");
    ctx.fillText = patchedFillText;
  }
  return ctx;
};
patchToString(patchedGetContext, "getContext");
HTMLCanvasElement.prototype.getContext = patchedGetContext;;
		} catch (e) {}
	;

		try {
			// Spoof AudioContext latency values to look like real hardware.
const audioLatencyAccessors = {
  get baseLatency() {
    return 0.005;
  },
  get outputLatency() {
    return 0.01;
  },
  get sampleRate() {
    return 48000;
  },
};

const defineAudioGetter = (proto, name) => {
  const descriptor = Object_getOwnPropertyDescriptor(audioLatencyAccessors, name);
  if (!descriptor || !descriptor.get) return;
  patchToString(descriptor.get, "get " + name);
  Object_defineProperty(proto, name, {
    get: descriptor.get,
    configurable: true,
    enumerable: true,
  });
};

const spoofLatency = (proto) => {
  defineAudioGetter(proto, "baseLatency");
  defineAudioGetter(proto, "outputLatency");
};

if (window.AudioContext) {
  spoofLatency(AudioContext.prototype);
}

if (window.OfflineAudioContext) {
  const OriginalOfflineAudioContext = window.OfflineAudioContext;
  const PatchedOfflineAudioContext = class OfflineAudioContext extends OriginalOfflineAudioContext {
    constructor(numberOfChannels, length, sampleRate) {
      super(numberOfChannels, length, sampleRate);

      const originalStartRendering = this.startRendering.bind(this);
      const patchedStartRendering = async function startRendering() {
        const buffer = await originalStartRendering();
        for (let c = 0; c < buffer.numberOfChannels; c++) {
          const channel = buffer.getChannelData(c);
          for (let i = 0; i < channel.length; i++) {
            if (Math_random() < 0.001) {
              channel[i] += (Math_random() - 0.5) * 1e-8;
            }
          }
        }
        return buffer;
      };
      patchToString(patchedStartRendering, "startRendering");
      this.startRendering = patchedStartRendering;
    }
  };
  patchToString(PatchedOfflineAudioContext, "OfflineAudioContext");
  window.OfflineAudioContext = PatchedOfflineAudioContext;
}

if (window.AudioContext) {
  defineAudioGetter(AudioContext.prototype, "sampleRate");
}
;
		} catch (e) {}
	;

		try {
			const locale = "en-US";
const languages = [locale, "en"];

const sameLanguages = (value) => {
  if (!value || value.length !== languages.length) return false;
  for (let index = 0; index < languages.length; index += 1) {
    if (value[index] !== languages[index]) return false;
  }
  return true;
};

const navigatorProto = Object_getPrototypeOf(navigator);
const navigatorAccessors = {
  get language() {
    return locale;
  },
  get languages() {
    return [locale, "en"];
  },
};

const defineNavigatorAccessor = (name) => {
  const owner = navigatorProto || navigator;
  const descriptor =
    (navigatorProto && Object_getOwnPropertyDescriptor(navigatorProto, name)) ||
    Object_getOwnPropertyDescriptor(navigator, name);
  if (descriptor && descriptor.configurable === false) return;

  const accessor = Object_getOwnPropertyDescriptor(navigatorAccessors, name);
  if (!accessor || !accessor.get) return;
  patchToString(accessor.get, "get " + name);

  Object_defineProperty(owner, name, {
    get: accessor.get,
    configurable: descriptor ? descriptor.configurable : true,
    enumerable: descriptor ? descriptor.enumerable : true,
  });

  const ownDescriptor = Object_getOwnPropertyDescriptor(navigator, name);
  if (owner !== navigator && ownDescriptor && ownDescriptor.configurable !== false) {
    Reflect_deleteProperty(navigator, name);
  }
};

if (navigator.language !== locale) {
  defineNavigatorAccessor("language");
}

if (!sameLanguages(navigator.languages)) {
  defineNavigatorAccessor("languages");
}
;
		} catch (e) {}
	;

		try {
			const mimeTypesData = [
	{
		type: "application/pdf",
		suffixes: "pdf",
		description: "",
		__pluginName: "Chrome PDF Viewer",
	},
	{
		type: "application/x-google-chrome-pdf",
		suffixes: "pdf",
		description: "Portable Document Format",
		__pluginName: "Chrome PDF Plugin",
	},
	{
		type: "application/x-nacl",
		suffixes: "",
		description: "Native Client Executable",
		__pluginName: "Native Client",
	},
	{
		type: "application/x-pnacl",
		suffixes: "",
		description: "Portable Native Client Executable",
		__pluginName: "Native Client",
	},
];

const pluginsData = [
	{
		name: "Chrome PDF Plugin",
		filename: "internal-pdf-viewer",
		description: "Portable Document Format",
		__mimeTypes: ["application/x-google-chrome-pdf"],
	},
	{
		name: "Chrome PDF Viewer",
		filename: "mhjfbmdgcfjbbpaeojofohoefgiehjai",
		description: "",
		__mimeTypes: ["application/pdf"],
	},
	{
		name: "Native Client",
		filename: "internal-nacl-plugin",
		description: "",
		__mimeTypes: ["application/x-nacl", "application/x-pnacl"],
	},
];

const defineProp = (obj, prop, value) =>
	Object_defineProperty(obj, prop, {
		value,
		writable: false,
		enumerable: false,
		configurable: true,
	});

const generateFunctionMocks = (proto, itemMainProp, dataArray) => {
	const item = new Window_Proxy(proto.item, {
		apply(target, ctx, args) {
			if (!args.length) {
				throw new TypeError(
					`Failed to execute 'item' on '${proto[Symbol.toStringTag]}': 1 argument required, but only 0 present.`,
				);
			}
			const isInteger = args[0] && Number.isInteger(Number(args[0]));
			return (isInteger ? dataArray[Number(args[0])] : dataArray[0]) || null;
		},
	});
	const namedItem = new Window_Proxy(proto.namedItem, {
		apply(target, ctx, args) {
			if (!args.length) {
				throw new TypeError(
					`Failed to execute 'namedItem' on '${proto[Symbol.toStringTag]}': 1 argument required, but only 0 present.`,
				);
			}
			return dataArray.find(item => item[itemMainProp] === args[0]) || null;
		},
	});
	patchToString(item, "item");
	patchToString(namedItem, "namedItem");
	const refresh = proto.refresh
		? new Window_Proxy(proto.refresh, {
				apply() {
					return undefined;
				},
			})
		: undefined;
	patchToString(refresh, "refresh");
	return { item, namedItem, refresh };
};

const generateMagicArray = (dataArray, proto, itemProto, itemMainProp) => {
	const makeItem = (data) => {
		const item = {};
		for (const prop of Object_keys(data)) {
			if (prop.startsWith("__")) continue;
			defineProp(item, prop, data[prop]);
		}
		return patchItem(item, data);
	};

	const patchItem = (item, data) => {
		let descriptor = Object_getOwnPropertyDescriptors(item);
		if (itemProto === Plugin.prototype) {
			descriptor = {
				...descriptor,
				length: {
					value: data.__mimeTypes.length,
					writable: false,
					enumerable: false,
					configurable: true,
				},
			};
		}

		const obj = Object_create(itemProto, descriptor);
		const blacklist = [...Object_keys(data), "length", "enabledPlugin"];
		return new Window_Proxy(obj, {
			ownKeys(target) {
				return [...Object_getOwnPropertyNames(target), ...Object.getOwnPropertySymbols(target)].filter(
					key => !blacklist.includes(key),
				);
			},
			getOwnPropertyDescriptor(target, prop) {
				if (blacklist.includes(prop)) return undefined;
				return Object_getOwnPropertyDescriptor(target, prop);
			},
		});
	};

	const magicArray = [];
	dataArray.forEach(data => {
		magicArray.push(makeItem(data));
	});

	magicArray.forEach(entry => {
		defineProp(magicArray, entry[itemMainProp], entry);
	});

	const magicArrayObj = Object_create(proto, {
		...Object_getOwnPropertyDescriptors(magicArray),
		length: {
			value: magicArray.length,
			writable: false,
			enumerable: false,
			configurable: true,
		},
	});

	const functionMocks = generateFunctionMocks(proto, itemMainProp, magicArray);

	return new Window_Proxy(magicArrayObj, {
		get(target, key = "") {
			if (key === "item") return functionMocks.item;
			if (key === "namedItem") return functionMocks.namedItem;
			if (proto === PluginArray.prototype && key === "refresh") return functionMocks.refresh;
			return Reflect_get(target, key);
		},
		ownKeys(target) {
			const keys = [];
			const typeProps = magicArray.map(entry => entry[itemMainProp]);
			typeProps.forEach((_, index) => keys.push(`${index}`));
			typeProps.forEach(propName => keys.push(propName));
			return keys;
		},
		getOwnPropertyDescriptor(target, prop) {
			if (prop === "length") return undefined;
			return Object_getOwnPropertyDescriptor(target, prop);
		},
	});
};

const generateMimeTypeArray = (data) =>
	generateMagicArray(data, MimeTypeArray.prototype, MimeType.prototype, "type");
const generatePluginArray = (data) =>
	generateMagicArray(data, PluginArray.prototype, Plugin.prototype, "name");

const mimeTypes = generateMimeTypeArray(mimeTypesData);
const plugins = generatePluginArray(pluginsData);

for (const pluginData of pluginsData) {
	pluginData.__mimeTypes.forEach((type, index) => {
		plugins[pluginData.name][index] = mimeTypes[type];
		Object_defineProperty(plugins[pluginData.name], type, {
			value: mimeTypes[type],
			writable: false,
			enumerable: false,
			configurable: true,
		});
		Object_defineProperty(mimeTypes[type], "enabledPlugin", {
			value:
				type === "application/x-pnacl"
					? mimeTypes["application/x-nacl"].enabledPlugin
					: new Window_Proxy(plugins[pluginData.name], {}),
			writable: false,
			enumerable: false,
			configurable: true,
		});
	});
}

const patchNavigator = (name, value) =>
	Object_defineProperty(Object_getPrototypeOf(navigator), name, {
		get() {
			return value;
		},
	});

if (!("plugins" in navigator) || navigator.plugins.length === 0) {
	patchNavigator("plugins", plugins);
	patchNavigator("mimeTypes", mimeTypes);
}
;
		} catch (e) {}
	;

		try {
			const hardwareConcurrencyName = "hardwareConcurrency";
let hardwareConcurrencyProto = Object_getPrototypeOf(navigator);
let hardwareConcurrencyOwner;
let hardwareConcurrencyDescriptor;

while (hardwareConcurrencyProto && !hardwareConcurrencyDescriptor) {
	hardwareConcurrencyDescriptor = Object_getOwnPropertyDescriptor(
		hardwareConcurrencyProto,
		hardwareConcurrencyName,
	);
	if (hardwareConcurrencyDescriptor) {
		hardwareConcurrencyOwner = hardwareConcurrencyProto;
	} else {
		hardwareConcurrencyProto = Object_getPrototypeOf(hardwareConcurrencyProto);
	}
}

const hardwareConcurrencyValue = 8;
let shouldPatchHardwareConcurrency = false;

if (
	hardwareConcurrencyOwner &&
	hardwareConcurrencyDescriptor &&
	hardwareConcurrencyDescriptor.configurable &&
	typeof hardwareConcurrencyDescriptor.get === "function"
) {
	shouldPatchHardwareConcurrency = true;
	try {
		shouldPatchHardwareConcurrency =
			Reflect_apply(hardwareConcurrencyDescriptor.get, navigator, []) !==
			hardwareConcurrencyValue;
	} catch {
		shouldPatchHardwareConcurrency = false;
	}
}

if (shouldPatchHardwareConcurrency) {
	const hardwareConcurrencyAccessors = {
		get hardwareConcurrency() {
			Reflect_apply(hardwareConcurrencyDescriptor.get, this, []);
			return hardwareConcurrencyValue;
		},
	};
	const getHardwareConcurrency = Object_getOwnPropertyDescriptor(
		hardwareConcurrencyAccessors,
		hardwareConcurrencyName,
	).get;

	if (typeof patchToString === "function") {
		patchToString(getHardwareConcurrency, "get hardwareConcurrency");
	}

	Object_defineProperty(hardwareConcurrencyOwner, hardwareConcurrencyName, {
		get: getHardwareConcurrency,
		set: hardwareConcurrencyDescriptor.set,
		enumerable: hardwareConcurrencyDescriptor.enumerable,
		configurable: hardwareConcurrencyDescriptor.configurable,
	});
}
;
		} catch (e) {}
	;

		try {
			const parseInput = (arg) => {
	const [mime, codecStr] = arg.trim().split(";");
	let codecs = [];
	if (codecStr && codecStr.includes('codecs="')) {
		codecs = codecStr
			.trim()
			.replace('codecs="', "")
			.replace('"', "")
			.trim()
			.split(",")
			.filter(Boolean)
			.map(item => item.trim());
	}
	return { mime, codecStr, codecs };
};

const originalCanPlayType = HTMLMediaElement.prototype.canPlayType;
const proxiedCanPlayType = new Window_Proxy(originalCanPlayType, {
	apply(target, ctx, args) {
		if (!args || !args.length) {
			return Reflect_apply(target, ctx, args);
		}
		const { mime, codecs } = parseInput(args[0]);
		if (mime === "video/mp4" && codecs.includes("avc1.42E01E")) {
			return "probably";
		}
		if (mime === "audio/x-m4a" && !codecs.length) {
			return "maybe";
		}
		if (mime === "audio/aac" && !codecs.length) {
			return "probably";
		}
		return Reflect_apply(target, ctx, args);
	},
});
patchToString(proxiedCanPlayType, "canPlayType");
Object_defineProperty(HTMLMediaElement.prototype, "canPlayType", {
	value: proxiedCanPlayType,
	writable: true,
	configurable: true,
	enumerable: true,
});
;
		} catch (e) {}
	;

		try {
			const patchWorkerConstructor = (name, OriginalWorker) => {
	if (typeof OriginalWorker !== "function") return;

	const windowDescriptor = Object_getOwnPropertyDescriptor(window, name);
	if (windowDescriptor && windowDescriptor.configurable === false) return;

	const NativeURL = window.URL;
	const URL_createObjectURL = NativeURL && NativeURL.createObjectURL;
	const URL_revokeObjectURL = NativeURL && NativeURL.revokeObjectURL;
	if (
		typeof NativeURL !== "function" ||
		typeof URL_createObjectURL !== "function" ||
		typeof URL_revokeObjectURL !== "function"
	) {
		return;
	}

	const sharedWorkerUrls = name === "SharedWorker" ? new Map() : undefined;

	const revokeUrl = (url) => {
		try {
			Reflect_apply(URL_revokeObjectURL, NativeURL, [url]);
		} catch {}
	};

	const scheduleWorkerUrlRevoke = (worker, url) => {
		let revoked = false;
		const revokeOnce = () => {
			if (revoked) return;
			revoked = true;
			revokeUrl(url);
		};
		try {
			if (typeof worker.addEventListener === "function") {
				Reflect_apply(worker.addEventListener, worker, ["error", revokeOnce, { once: true }]);
			}
		} catch {}
		Window_setTimeout(revokeOnce, 1000);
	};

	if (sharedWorkerUrls) {
		try {
			window.addEventListener("pagehide", (event) => {
				if (event && event.persisted) return;
				for (const url of sharedWorkerUrls.values()) {
					revokeUrl(url);
				}
				sharedWorkerUrls.clear();
			});
		} catch {}
	}

	const canWrapArguments = (args) => {
		if (!args || args.length !== 1) {
			return name === "SharedWorker" && args && args.length === 2 && typeof args[1] === "string";
		}
		return true;
	};

	const resolveWorkerUrl = (scriptURL) => {
		const baseUrl = document.baseURI || window.location.href;
		const scriptUrlString =
			typeof scriptURL === "string"
				? scriptURL
				: scriptURL instanceof NativeURL
					? scriptURL.href
					: undefined;
		if (scriptUrlString === undefined) return undefined;
		let absoluteUrl;
		try {
			absoluteUrl = new NativeURL(scriptUrlString, baseUrl);
		} catch {
			return undefined;
		}
		if (absoluteUrl.origin !== window.location.origin) return undefined;
		if (absoluteUrl.protocol !== "http:" && absoluteUrl.protocol !== "https:") return undefined;
		if (absoluteUrl.username || absoluteUrl.password) return undefined;
		return absoluteUrl.href;
	};

	const buildWorkerPrelude = () => {
		const values = [];
		const addNavigatorString = (prop) => {
			try {
				const value = navigator[prop];
				if (typeof value === "string") values.push([prop, value]);
			} catch {}
		};
		addNavigatorString("userAgent");
		addNavigatorString("platform");
		if (!values.length) return "";

		return `(() => {
	try {
		const values = ${JSON.stringify(values)};
		const nav = self.navigator;
		if (!nav) return;
		const defineProperty = Object.defineProperty;
		const getOwnPropertyDescriptor = Object.getOwnPropertyDescriptor;
		const getPrototypeOf = Object.getPrototypeOf;
		const Reflect_apply = Reflect.apply;
		const nativeFunctionToString = Function.prototype.toString;
		const nativeSources = new WeakMap();
		const WeakMap_get = WeakMap.prototype.get;
		const WeakMap_has = WeakMap.prototype.has;
		const WeakMap_set = WeakMap.prototype.set;
		const rememberNative = (fn, source) => {
			if (typeof fn === "function") Reflect_apply(WeakMap_set, nativeSources, [fn, source]);
			return fn;
		};
		const findDescriptor = (prop) => {
			let owner = nav;
			while (owner) {
				const descriptor = getOwnPropertyDescriptor(owner, prop);
				if (descriptor) return [owner, descriptor];
				owner = getPrototypeOf(owner);
			}
			return [getPrototypeOf(nav) || nav, undefined];
		};
		let patched = false;
		for (let i = 0; i < values.length; i += 1) {
			const prop = values[i][0];
			const value = values[i][1];
			try {
				if (nav[prop] === value) continue;
			} catch (_) {}
			const found = findDescriptor(prop);
			const owner = found[0];
			const descriptor = found[1];
			if (!owner || (descriptor && descriptor.configurable === false)) continue;
			const getterDescriptor = getOwnPropertyDescriptor({ get [prop]() { return value; } }, prop);
			const getter = getterDescriptor && getterDescriptor.get;
			if (typeof getter !== "function") continue;
			rememberNative(getter, "function get " + prop + "() { [native code] }");
			defineProperty(owner, prop, {
				get: getter,
				enumerable: descriptor ? descriptor.enumerable : true,
				configurable: true,
			});
			patched = true;
		}
		if (!patched) return;
		const toStringDescriptor = getOwnPropertyDescriptor(Function.prototype, "toString");
		if (toStringDescriptor && toStringDescriptor.configurable === false) return;
		const functionToString = new Proxy(nativeFunctionToString, {
			apply(target, thisArg, args) {
				if (Reflect_apply(WeakMap_has, nativeSources, [thisArg])) return Reflect_apply(WeakMap_get, nativeSources, [thisArg]);
				return Reflect_apply(target, thisArg, args);
			},
		});
		rememberNative(functionToString, "function toString() { [native code] }");
		defineProperty(Function.prototype, "toString", {
			value: functionToString,
			writable: toStringDescriptor ? toStringDescriptor.writable : true,
			configurable: toStringDescriptor ? toStringDescriptor.configurable : true,
			enumerable: toStringDescriptor ? toStringDescriptor.enumerable : false,
		});
	} catch (_) {}
})();`;
	};

	const buildWrappedUrl = (scriptURL) => {
		const originalUrl = resolveWorkerUrl(scriptURL);
		if (!originalUrl) return undefined;
		if (sharedWorkerUrls) {
			const cachedUrl = sharedWorkerUrls.get(originalUrl);
			if (cachedUrl) return { url: cachedUrl, cacheKey: originalUrl };
		}

		const prelude = buildWorkerPrelude();
		if (!prelude) return undefined;

		try {
			const blob = new Window_Blob(
				[prelude, "\n", `importScripts(${JSON.stringify(originalUrl)});`],
				{ type: "application/javascript" },
			);
			const url = Reflect_apply(URL_createObjectURL, NativeURL, [blob]);
			if (sharedWorkerUrls) sharedWorkerUrls.set(originalUrl, url);
			return { url, cacheKey: sharedWorkerUrls ? originalUrl : undefined };
		} catch {
			return undefined;
		}
	};

	const handler = {
		construct(target, args, newTarget) {
			if (!canWrapArguments(args)) {
				return Reflect_construct(target, args || [], newTarget);
			}

			const wrapped = buildWrappedUrl(args[0]);
			if (!wrapped) {
				return Reflect_construct(target, args || [], newTarget);
			}

			const wrappedArgs = [wrapped.url];
			for (let i = 1; i < args.length; i += 1) {
				wrappedArgs[i] = args[i];
			}

			try {
				const worker = Reflect_construct(target, wrappedArgs, newTarget);
				if (!sharedWorkerUrls) {
					scheduleWorkerUrlRevoke(worker, wrapped.url);
				}
				return worker;
			} catch {
				if (wrapped.cacheKey && sharedWorkerUrls) sharedWorkerUrls.delete(wrapped.cacheKey);
				revokeUrl(wrapped.url);
				return Reflect_construct(target, args || [], newTarget);
			}
		},
		apply(target, thisArg, args) {
			return Reflect_apply(target, thisArg, args || []);
		},
	};

	const proxied = new Window_Proxy(OriginalWorker, handler);
	patchToString(proxied, name);
	Object_defineProperty(window, name, {
		value: proxied,
		writable:
			windowDescriptor && "writable" in windowDescriptor
				? windowDescriptor.writable
				: true,
		configurable: windowDescriptor ? windowDescriptor.configurable : true,
		enumerable: windowDescriptor ? windowDescriptor.enumerable : false,
	});
};

try {
	patchWorkerConstructor("Worker", window.Worker);
	patchWorkerConstructor("SharedWorker", window.SharedWorker);
} catch {}
;
		} catch (e) {}
	
				} finally {
					if (iframe && iframe.parentNode) iframe.parentNode.removeChild(iframe);
				}})();