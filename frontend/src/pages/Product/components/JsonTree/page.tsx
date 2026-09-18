import * as S from './styles'

type JsonTreeProps = {
  value: Record<string, unknown>
}

type JsonNodeProps = {
  name: string
  value: unknown
  path: string
}

function isCollection(value: unknown): value is Record<string, unknown> | unknown[] {
  return value !== null && typeof value === 'object'
}

function formatValue(value: unknown): string {
  if (value === null) {
    return 'null'
  }
  if (typeof value === 'string') {
    return JSON.stringify(value)
  }
  if (typeof value === 'undefined') {
    return 'undefined'
  }
  return String(value)
}

function JsonNode({ name, value, path }: JsonNodeProps) {
  if (!isCollection(value)) {
    return (
      <S.Primitive>
        <S.Key>{name}</S.Key>
        <S.Separator>:</S.Separator>
        <S.Value>{formatValue(value)}</S.Value>
      </S.Primitive>
    )
  }

  const entries = Array.isArray(value)
    ? value.map((item, index) => [`[${index}]`, item] as const)
    : Object.entries(value)
  const label = Array.isArray(value) ? `[${entries.length}]` : `{${entries.length}}`

  return (
    <S.Node>
      <S.NodeSummary>
        <S.Key>{name}</S.Key>
        <S.CollectionSize>{label}</S.CollectionSize>
      </S.NodeSummary>
      <S.NodeChildren>
        {entries.length === 0 ? (
          <S.EmptyCollection>{Array.isArray(value) ? '[]' : '{}'}</S.EmptyCollection>
        ) : (
          entries.map(([childName, childValue]) => (
            <JsonNode
              key={`${path}.${childName}`}
              name={childName}
              value={childValue}
              path={`${path}.${childName}`}
            />
          ))
        )}
      </S.NodeChildren>
    </S.Node>
  )
}

export function JsonTree({ value }: JsonTreeProps) {
  return (
    <S.Tree>
      {Object.entries(value).map(([name, childValue]) => (
        <JsonNode key={name} name={name} value={childValue} path={name} />
      ))}
    </S.Tree>
  )
}
