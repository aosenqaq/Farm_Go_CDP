/*
 * 原创脚本开发/抄袭/Faker/倒卖全家三日内暴毙，一辈子发不了财，开源项目地址：[aosenqaq/QQ_Farm_CDP_Manger](https://github.com/aosenqaq/QQ_Farm_CDP_Manger)
 * 你是自己不会写脚本？？天天抄袭我？？
 */

// 水印：二开倒卖先别急，README 都没看明白就上链接，属实有点绷不住。
const getMainModule = (version, runtimeTarget) => {
    if (runtimeTarget === "yyb") {
        return Process.findModuleByName("flue.dll");
    }
    if (version >= 13331) {
        return Process.findModuleByName("flue.dll");
    }
    return Process.findModuleByName("WeChatAppEx.exe");
};

// 1000: from issue #83 <-- will crash the process
const BLOCKED_PATCH_SCENE_NUMBERS = [1000];

// 1023: from Windows desktop shortcut
// 1007: from issue #80
// 1008: from issue #53
// 1027: from issue #78
// 1035: from issue #78
// 1053: from issue #25
// 1074: from issue #32
// 1145: from search
// 1178: from phone (issue #117)
// 1256: from recent
// 1260: from frequently used
// 1302: from services
// 1308: minigame?
const DEFAULT_PATCH_SCENE_NUMBERS = [
    1005, 1007, 1008, 1023, 1027, 1035, 1053, 1074, 1145, 1178, 1256, 1260,
    1302, 1308,
];

const normalizeSceneNumbers = (value) => {
    if (!Array.isArray(value)) {
        return [];
    }
    const result = [];
    value.forEach((item) => {
        const number = Number(item);
        if (Number.isFinite(number)) {
            result.push(number);
        }
    });
    return result;
};

const getPatchSceneNumbers = (config) => {
    const debugScenes = normalizeSceneNumbers(config.DebugScenes);
    if (debugScenes.length > 0) {
        return debugScenes.filter((scene, index) => {
            return debugScenes.indexOf(scene) === index && BLOCKED_PATCH_SCENE_NUMBERS.indexOf(scene) === -1;
        });
    }
    const configured = normalizeSceneNumbers(config.SceneWhitelist);
    const extra = normalizeSceneNumbers(config.ExtraPatchSceneNumbers);
    const source = configured.length > 0
        ? configured.concat(extra)
        : DEFAULT_PATCH_SCENE_NUMBERS.concat(extra);
    const blocked = BLOCKED_PATCH_SCENE_NUMBERS;
    return source.filter((scene, index) => {
        return source.indexOf(scene) === index && blocked.indexOf(scene) === -1;
    });
};

const expandSceneOffsets = (config) => {
    if (Array.isArray(config.SceneOffsets) &&
        (config.SceneOffsets.length === 2 || config.SceneOffsets.length >= 6 || config.RuntimeTarget === "yyb")) {
        return config.SceneOffsets;
    }
    const [configOffset, sceneRootOffset, sceneValueOffset] = config.ScenePathOffsets || [56, 8, 16];
    return [
        configOffset,
        config.SceneOffsets[0],
        sceneRootOffset,
        config.SceneOffsets[1],
        sceneValueOffset,
        config.SceneOffsets[2],
    ];
};

const isMemoryRangeAccessible = (address, size, access) => {
    try {
        const range = Process.findRangeByAddress(address);
        if (!range || range.protection.indexOf(access) === -1) {
            return false;
        }
        return address.compare(range.base) >= 0 &&
            address.add(size).compare(range.base.add(range.size)) <= 0;
    } catch (_) {
        return false;
    }
};

const readPointerIfAccessible = (address) => {
    if (!isMemoryRangeAccessible(address, Process.pointerSize, "r")) {
        return null;
    }
    try {
        const value = address.readPointer();
        return value.isNull() ? null : value;
    } catch (_) {
        return null;
    }
};

const readIntIfAccessible = (address) => {
    if (!isMemoryRangeAccessible(address, 4, "r")) {
        return null;
    }
    try {
        return address.readS32();
    } catch (_) {
        return null;
    }
};

const readU32IfAccessible = (address) => {
    if (!isMemoryRangeAccessible(address, 4, "r")) {
        return null;
    }
    try {
        return address.readU32();
    } catch (_) {
        return null;
    }
};

const patchCDPFilter = (base, config) => {
    // xref: SendToClientFilter OR devtools_message_filter_applet_webview.cc
    const offset = config.CDPFilterHookOffset;
    Interceptor.attach(base.add(offset), {
        onEnter(args) {
            const inputValue = readPointerIfAccessible(args[0]);
            if (!inputValue) {
                this.inputValue = null;
                return;
            }
            send(
                `[patch] CDP filter on enter, original value of input: ${inputValue}`,
            );
            this.inputValue = inputValue;
        },
        onLeave(retval) {
            const inputValue = this.inputValue;
            const filterValue = inputValue && readU32IfAccessible(inputValue.add(8));
            if (filterValue === null) {
                return;
            }

            send(
                `[patch] CDP filter on leave, patch input, now value: ${inputValue}; ` +
                    `*(input + 8) = ${filterValue}`,
            );
            if (filterValue === 6 && isMemoryRangeAccessible(inputValue.add(8), 4, "w")) {
                inputValue.add(8).writeU32(0x0);
            }
        },
    });
};

const patchDebugWebSocketURL = (passArgs, config) => {
    const url = config.DebugWebSocketURL;
    if (!url || !passArgs || passArgs.isNull()) {
        return;
    }
    try {
        const websocketConfigPtr = readPointerIfAccessible(passArgs.add(8));
        if (!websocketConfigPtr) {
            send("[hook] debug websocket config pointer is null");
            return;
        }
        const websocketServerStringPtr = websocketConfigPtr.add(520);
        if (!isMemoryRangeAccessible(websocketServerStringPtr, url.length + 1, "rw")) {
            send("[hook] debug websocket url pointer is not writable");
            return;
        }
        const original = websocketServerStringPtr.readUtf8String();
        if (!original) {
            send("[hook] debug websocket url is empty");
            return;
        }
        if (url.length > original.length) {
            send(`[hook] debug websocket url too long, original: ${original}, target: ${url}`);
            return;
        }
        send(`[hook] hook websocket server, original: ${original}, target: ${url}`);
        websocketServerStringPtr.writeUtf8String(url);
    } catch (error) {
        send(`[hook] hook websocket server failed: ${error}`);
    }
};

const resolveLoadStartPassArgs = (a1, config) => {
    const sceneOffsets = expandSceneOffsets(config);
    // A two-offset path ends at a direct scene number, not a pass-args object.
    if (sceneOffsets.length < 3) {
        return ptr(0);
    }
    const sceneRoot = readPointerIfAccessible(a1.add(sceneOffsets[0]));
    if (!sceneRoot) {
        return ptr(0);
    }
    const passArgs = readPointerIfAccessible(sceneRoot.add(sceneOffsets[1]));
    if (!passArgs) {
        return ptr(0);
    }
    return passArgs;
};

const hookOnLoadScene = (a1, config) => {
    const sceneOffsets = expandSceneOffsets(config);
    const passArgs = resolveLoadStartPassArgs(a1, config);
    let miniappScenePtr = a1;
    for (let index = 0; index < sceneOffsets.length; index += 1) {
        miniappScenePtr = miniappScenePtr.add(sceneOffsets[index]);
        if (index < sceneOffsets.length - 1) {
            const nextPointer = readPointerIfAccessible(miniappScenePtr);
            if (!nextPointer) {
                send("[hook] scene pointer is not readable");
                return;
            }
            miniappScenePtr = nextPointer;
        }
    }
    const scene = readIntIfAccessible(miniappScenePtr);
    if (scene === null) {
        send("[hook] scene value is not readable");
        return;
    }
    send(`[hook] scene: ${scene}`);

    const sceneNumberArray = getPatchSceneNumbers(config);
    if (sceneNumberArray.indexOf(scene) === -1) {
        return;
    }
    if (!isMemoryRangeAccessible(miniappScenePtr, 4, "w")) {
        send("[hook] scene value is not writable");
        return;
    }
    send("[hook] hook scene condition -> 1101");
    miniappScenePtr.writeS32(1101);
    patchDebugWebSocketURL(passArgs, config);
};

const patchOnLoadStart = (base, config) => {
    // xref: AppletIndexContainer::OnLoadStart
    Interceptor.attach(base.add(config.LoadStartHookOffset), {
        onEnter(args) {
            send(
                `[inteceptor] AppletIndexContainer::OnLoadStart onEnter, ` +
                    `indexContainer.this: ${this.context.rcx}`,
            );
            // write dl to 0x1
            if ((this.context.rdx & 0xff) !== 1) {
                this.context.rdx = (this.context.rdx & ~0xff) | 0x1;
            }
            // handle onLoad scene
            hookOnLoadScene(this.context.rcx, config);
        },
        onLeave(retval) {
            // do nothing
        },
    });
};

const parseConfig = () => {
    const rawConfig = `@@CONFIG@@`;
    if (rawConfig.indexOf("@@") !== -1) {
        // test addresses
        return {
            Version: 18955,
            LoadStartHookOffset: "0x25B52C0",
            CDPFilterHookOffset: "0x30248B0",
            SceneOffsets: [1408, 1344, 488],
            ScenePathOffsets: [56, 8, 16],
            SceneWhitelist: DEFAULT_PATCH_SCENE_NUMBERS,
            RuntimeTarget: "cdp",
        };
    }
    return JSON.parse(rawConfig);
};

const main = () => {
    const config = parseConfig();
    const mainModule = getMainModule(config.Version, config.RuntimeTarget);
    patchOnLoadStart(mainModule.base, config);
    patchCDPFilter(mainModule.base, config);
};

main();
