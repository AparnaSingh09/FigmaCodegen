import { useState } from 'react'
import './App.css'

const BACKEND_URL = 'http://localhost:8081'

function App() {
  const [figmaUrl, setFigmaUrl] = useState('')
  const [framework, setFramework] = useState('react')
  const [code, setCode] = useState('')
  const [error, setError] = useState('')
  const [loading, setLoading] = useState(false)
  const [copied, setCopied] = useState(false)

  async function handleSubmit(e) {
    e.preventDefault()
    setLoading(true)
    setError('')
    setCode('')
    setCopied(false)

    try {
      const res = await fetch(`${BACKEND_URL}/api/generate`, {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ figmaUrl, framework }),
      })
      const data = await res.json()
      if (data.error) {
        setError(data.error)
      } else {
        setCode(data.code)
      }
    } catch {
      setError('Could not reach the backend - is it running on :8081?')
    } finally {
      setLoading(false)
    }
  }

  function handleCopy() {
    navigator.clipboard.writeText(code)
    setCopied(true)
    setTimeout(() => setCopied(false), 1500)
  }

  return (
    <div className="page">
      <h1>FigmaCodegen</h1>
      <p className="subtitle">Paste a Figma frame link, get generated code.</p>

      <form onSubmit={handleSubmit} className="form">
        <input
          type="text"
          placeholder="https://www.figma.com/design/.../File-Name?node-id=1-23"
          value={figmaUrl}
          onChange={(e) => setFigmaUrl(e.target.value)}
          required
        />
        <select value={framework} onChange={(e) => setFramework(e.target.value)}>
          <option value="react">React</option>
          <option value="html">HTML</option>
        </select>
        <button type="submit" disabled={loading}>
          {loading ? 'Generating...' : 'Generate code'}
        </button>
      </form>

      {error && <p className="error">{error}</p>}

      {code && (
        <div className="result">
          <div className="result-header">
            <span>Generated code</span>
            <button type="button" onClick={handleCopy}>
              {copied ? 'Copied!' : 'Copy'}
            </button>
          </div>
          <pre>
            <code>{code}</code>
          </pre>
        </div>
      )}
    </div>
  )
}

export default App
