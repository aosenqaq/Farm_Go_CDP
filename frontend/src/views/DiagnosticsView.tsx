import { Play } from 'lucide-react';
import { useEffect, useState } from 'react';

import { RunDiagnostic, RuntimeLinkStatus } from '../../wailsjs/go/desktop/App';

type DiagnosticsViewProps = {
  compact?: boolean;
};

export function DiagnosticsView({ compact = false }: DiagnosticsViewProps) {
  const [method, setMethod] = useState('host.describe');
  const [params, setParams] = useState('{}');
  const [result, setResult] = useState('等待执行');
  const [error, setError] = useState('');
  const [target, setTarget] = useState('qq_ws');

  useEffect(() => {
    RuntimeLinkStatus()
      .then((status) => setTarget(status.target || 'qq_ws'))
      .catch(console.error);
  }, []);

  async function run() {
    setError('');
    setResult('执行中...');

    let parsed: Record<string, unknown>;
    try {
      parsed = JSON.parse(params);
    } catch {
      setResult('等待执行');
      setError('参数不是合法 JSON');
      return;
    }

    const response = await RunDiagnostic(method, parsed);
    setResult(JSON.stringify(response, null, 2));
  }

  return (
    <section className={compact ? 'diagnostics-surface compact-tool-surface' : 'view-stack fill'}>
      {!compact && (
        <header className="page-header">
          <div>
            <h1>诊断</h1>
            <p>执行当前链路诊断调用</p>
          </div>
        </header>
      )}

      <div className="diagnostic-layout">
        <section className="form-panel">
          <div className="form-context">
            <span>当前链路</span>
            <strong>{labelForTarget(target)}</strong>
          </div>
          <label>
            <span>方法</span>
            <select value={method} onChange={(event) => setMethod(event.target.value)}>
              <option value="host.describe">host.describe</option>
              <option value="gameCtl.probe">gameCtl.probe</option>
            </select>
          </label>

          <label>
            <span>参数 JSON</span>
            <textarea value={params} onChange={(event) => setParams(event.target.value)} spellCheck={false} />
          </label>

          {error ? <div className="inline-error">{error}</div> : null}

          <button className="primary-button" type="button" onClick={run}>
            <Play size={17} fill="currentColor" />
            <span>执行</span>
          </button>
        </section>

        <section className="result-panel">
          <div className="panel-title">结果输出</div>
          <pre>{result}</pre>
        </section>
      </div>
    </section>
  );
}

function labelForTarget(target?: string) {
  if (target === 'wechat_cdp') return '微信 CDP';
  if (target === 'yyb_cdp') return '应用宝 CDP';
  return 'QQ WS';
}
