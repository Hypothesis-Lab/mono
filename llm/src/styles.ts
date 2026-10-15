import { createGlobalStyle, css, styled } from 'styled-components'

type QuestionStatus = 'idle' | 'running' | 'complete' | 'error'

export const GlobalStyle = createGlobalStyle`
  :root {
    font-family: Inter, ui-sans-serif, system-ui, -apple-system, BlinkMacSystemFont, "Segoe UI", sans-serif;
    color: #f4f3ee;
    background: #1a1c18;
    font-synthesis: none;
    text-rendering: optimizeLegibility;
    -webkit-font-smoothing: antialiased;
  }

  *, *::before, *::after { box-sizing: border-box; }
  html, body, #root { min-width: 320px; min-height: 100%; margin: 0; }
  body { min-height: 100vh; }
  button { font: inherit; }
`

export const AppShell = styled.main`
  display: flex;
  min-height: 100vh;
  background: #1a1c18;

  @media (max-width: 768px) { display: block; }
`

export const Sidebar = styled.aside`
  display: flex;
  width: 260px;
  flex-direction: column;
  flex-shrink: 0;
  padding: 28px 16px 20px;
  border-right: 1px solid #383b33;
  background: #20221e;

  @media (max-width: 768px) {
    width: 100%;
    padding: 18px 14px 14px;
    border-right: 0;
    border-bottom: 1px solid #383b33;
  }
`

export const Brand = styled.div`
  display: flex;
  align-items: center;
  gap: 10px;
  padding: 0 12px 42px;
  color: #f4f3ee;
  font-size: 17px;
  font-weight: 650;
  letter-spacing: -0.02em;

  @media (max-width: 768px) { padding: 0 8px 20px; }
`

export const BrandMark = styled.span`
  display: grid;
  width: 27px;
  height: 27px;
  place-items: center;
  border-radius: 8px;
  color: #10110f;
  background: #d5f36b;
  font-size: 17px;
  font-weight: 800;
`

export const Eyebrow = styled.p`
  margin: 0 0 13px;
  color: #9baf68;
  font-size: 11px;
  font-weight: 700;
  letter-spacing: 0.12em;
  text-transform: uppercase;
`

export const SidebarLabel = styled.div`
  padding: 0 12px 10px;
  color: #858a7e;
  font-size: 11px;
  font-weight: 700;
  letter-spacing: 0.12em;
  text-transform: uppercase;

  @media (max-width: 768px) { display: none; }
`

export const PipelineButton = styled.div<{ $active?: boolean }>`
  display: flex;
  width: 100%;
  align-items: flex-start;
  gap: 11px;
  padding: 12px;
  border: 1px solid ${({ $active }) => $active ? '#373c2c' : 'transparent'};
  border-radius: 9px;
  color: ${({ $active }) => $active ? '#f4f3ee' : '#aaafa5'};
  background: ${({ $active }) => $active ? '#20241a' : 'transparent'};
  text-align: left;

  strong, small { display: block; }
  strong { font-size: 13px; font-weight: 600; }
  small { margin-top: 4px; color: #858a7e; font-size: 11px; }

  @media (max-width: 768px) { display: none; }
`

export const PipelineDot = styled.i`
  width: 7px;
  height: 7px;
  flex-shrink: 0;
  margin-top: 5px;
  border-radius: 50%;
  background: #d5f36b;
  box-shadow: 0 0 0 3px rgba(213, 243, 107, 0.1);
`

export const SidebarFooter = styled.div`
  display: flex;
  align-items: center;
  gap: 9px;
  margin-top: auto;
  padding: 12px;
  color: #858a7e;
  font-size: 11px;

  @media (max-width: 768px) { display: none; }
`

export const StatusDot = styled.i`
  width: 6px;
  height: 6px;
  border-radius: 50%;
  background: #87d5a2;
`

export const Workspace = styled.section`
  display: flex;
  width: 0;
  min-width: 0;
  height: 100vh;
  flex: 1;
  flex-direction: column;
  padding: 20px 24px 14px;

  @media (max-width: 768px) {
    width: 100%;
    height: calc(100vh - 60px);
    padding: 14px 14px 10px;
  }
`

export const Topbar = styled.header`
  display: flex;
  align-items: flex-end;
  justify-content: space-between;
  gap: 24px;
  flex-shrink: 0;
  padding-bottom: 12px;
`

export const GraphPanel = styled.section`
  position: relative;
  min-height: 0;
  flex: 1;
  overflow: hidden;
  border: 1px solid #383b33;
  border-radius: 12px;
  background: #1c1f1a;

  .react-flow { --xy-background-color: #1c1f1a; }
  .react-flow__attribution { display: none; }
  .react-flow__controls {
    overflow: hidden;
    border: 1px solid #34382e;
    border-radius: 7px;
    box-shadow: none;
  }
  .react-flow__controls-button {
    width: 28px;
    height: 28px;
    border-bottom: 1px solid #34382e;
    color: #a8ad9e;
    background: #191c17;
    fill: #a8ad9e;
  }
  .react-flow__controls-button:last-child { border-bottom: 0; }
  .react-flow__minimap {
    right: 14px;
    bottom: 14px;
    overflow: hidden;
    border: 1px solid #34382e;
    border-radius: 7px;
    background: #191c17;
  }
`

export const GraphLegend = styled.div`
  position: absolute;
  z-index: 5;
  top: 16px;
  left: 18px;
  display: flex;
  gap: 16px;
  padding: 8px 10px;
  border: 1px solid #292d25;
  border-radius: 6px;
  color: #858a7e;
  background: rgba(17, 19, 15, 0.9);
  font-size: 10px;

  span { display: flex; align-items: center; gap: 6px; }
  @media (max-width: 768px) {
    right: 12px;
    left: 12px;
    justify-content: space-between;
    gap: 8px;
  }
`

export const LegendDot = styled.i<{ $kind: 'input' | 'question' | 'complete' }>`
  display: block;
  width: 6px;
  height: 6px;
  border-radius: 50%;
  background: ${({ $kind }) => $kind === 'input' ? '#d5f36b' : $kind === 'complete' ? '#9ed36d' : '#74815a'};
`

export const GraphActions = styled.div`
  display: flex;
  min-height: 52px;
  align-items: center;
  justify-content: flex-end;
  flex-shrink: 0;
  gap: 20px;
  padding: 8px 2px 0;

  @media (max-width: 768px) { align-items: flex-start; flex-direction: column; }
`

export const RunButton = styled.button`
  display: flex;
  align-items: center;
  gap: 10px;
  padding: 10px 14px;
  border: 1px solid #d5f36b;
  border-radius: 7px;
  color: #11130e;
  background: #d5f36b;
  font-size: 12px;
  font-weight: 700;
  cursor: pointer;

  &:hover:not(:disabled) { background: #e0fb87; }
  &:disabled { cursor: wait; opacity: 0.65; }
`

const statusBorder = (status: QuestionStatus) => {
  if (status === 'running') return '#c0d862'
  if (status === 'complete') return '#71a968'
  if (status === 'error') return '#a8584c'
  return '#454d3b'
}

export const GraphNode = styled.div<{ $status?: QuestionStatus; $kind?: 'product' | 'audience' | 'catalog' }>`
  position: relative;
  width: ${({ $kind }) => $kind === 'product' ? '320px' : $kind === 'audience' ? '460px' : $kind === 'catalog' ? '380px' : '250px'};
  padding: 17px;
  border: 1px solid ${({ $kind, $status = 'idle' }) => $kind === 'product' ? '#ac754a' : ($kind === 'audience' || $kind === 'catalog') && $status === 'idle' ? '#758b4c' : statusBorder($status)};
  border-radius: 10px;
  color: #e9eee1;
  background: ${({ $kind, $status }) => {
    if ($kind === 'product') return '#211b16'
    if ($kind === 'audience' || $kind === 'catalog') return '#1b2117'
    if ($status === 'complete') return '#182118'
    if ($status === 'error') return '#241816'
    return '#1a1f17'
  }};
  box-shadow: 0 10px 24px rgba(0, 0, 0, 0.18);
`

export const GraphHandle = styled.div`
  width: 8px;
  height: 8px;
  border: 2px solid #10110f;
  background: #aebd83;
`

export const NodeEyebrow = styled.div<{ $kind?: 'product' | 'question' }>`
  color: ${({ $kind }) => $kind === 'product' ? '#d39a6c' : '#a7b084'};
  font-size: 9px;
  font-weight: 700;
  letter-spacing: 0.14em;
  text-transform: uppercase;
`

export const NodeTitle = styled.div`
  margin-top: 6px;
  color: #f0f4e7;
  font-size: 16px;
  font-weight: 650;
  letter-spacing: -0.02em;
`

export const ProductInput = styled.textarea`
  display: block;
  width: 100%;
  min-height: 112px;
  margin-top: 12px;
  padding: 9px;
  resize: vertical;
  border: 1px solid #5e4938;
  border-radius: 6px;
  outline: none;
  color: #e9dfd3;
  background: #171411;
  font: inherit;
  font-size: 11px;
  line-height: 1.45;

  &:focus { border-color: #d39a6c; }
`

export const QuestionHeader = styled.div`
  display: flex;
  align-items: flex-start;
  justify-content: space-between;
  gap: 10px;
`

export const NodeStatus = styled.span<{ $status: QuestionStatus }>`
  color: ${({ $status }) => $status === 'running' ? '#d5f36b' : $status === 'complete' ? '#9ed36d' : $status === 'error' ? '#f09a8a' : '#747d6a'};
  font-size: 9px;
  text-transform: uppercase;
`

export const QuestionCopy = styled.div`
  min-height: 48px;
  margin-top: 11px;
  color: #9fa69a;
  font-size: 11px;
  line-height: 1.45;
`

export const Answer = styled.div<{ $visible?: boolean; $error?: boolean; $ranking?: boolean }>`
  min-height: 43px;
  margin-top: 13px;
  padding: 9px;
  border: 1px ${({ $visible }) => $visible ? 'solid' : 'dashed'} ${({ $error, $visible }) => $error ? '#75443d' : $visible ? '#526c40' : '#3b4534'};
  border-radius: 6px;
  color: ${({ $error, $visible }) => $error ? '#f09a8a' : $visible ? '#dce9d1' : '#697365'};
  background: ${({ $error, $visible }) => $error ? '#201514' : $visible ? '#182018' : '#141814'};
  font-size: 11px;
  line-height: 1.45;
  ${({ $ranking }) => $ranking && css`white-space: pre-wrap;`}
`

export const AudienceToggle = styled.button`
  display: flex;
  width: 100%;
  align-items: center;
  justify-content: space-between;
  margin-top: 12px;
  padding: 9px 10px;
  border: 1px solid #45513a;
  border-radius: 6px;
  color: #dce9d1;
  background: #20281c;
  font-size: 11px;
  text-align: left;
  cursor: pointer;

  &:hover { border-color: #81945c; background: #283321; }
  &:focus-visible { outline: 2px solid #d5f36b; outline-offset: 2px; }
`

export const AudienceList = styled.div`
  display: grid;
  max-height: 380px;
  gap: 8px;
  overflow-y: auto;
  margin-top: 10px;
  padding-right: 4px;
  scrollbar-color: #59634b #171b15;
  scrollbar-width: thin;
`

export const AudienceEntry = styled.article`
  padding: 10px;
  border: 1px solid #343e2d;
  border-radius: 6px;
  background: #171c15;

  strong {
    display: block;
    color: #e5ebdc;
    font-size: 11px;
    line-height: 1.4;
  }

  p {
    margin: 5px 0 0;
    color: #a4ad9c;
    font-size: 10px;
    line-height: 1.45;
    white-space: normal;
  }
`

export const QuestionEdgeStyles = createGlobalStyle`
  .question-edge .react-flow__edge-path { stroke: #59634b; stroke-width: 1.5; }
  .question-edge.running .react-flow__edge-path { stroke: #c0d862; stroke-dasharray: 5 4; }
  .question-edge.complete .react-flow__edge-path { stroke: #8abf68; stroke-width: 2; }
  .question-edge.error .react-flow__edge-path { stroke: #b45e51; }
`
