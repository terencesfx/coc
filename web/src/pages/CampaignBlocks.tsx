import { useEffect, useState, type FormEvent } from "react";
import ReactMarkdown from "react-markdown";
import remarkGfm from "remark-gfm";
import {
  APIError,
  api,
  type CampaignAsset,
  type CampaignBlock,
} from "../api/client";

export function CampaignBlocks({
  campaignID,
  canManage,
  playerPreview = false,
}: {
  campaignID: string;
  canManage: boolean;
  playerPreview?: boolean;
}) {
  const [items, setItems] = useState<CampaignBlock[]>([]);
  const [type, setType] = useState<"heading" | "text" | "clue" | "image">(
    "text",
  );
  const [title, setTitle] = useState("");
  const [content, setContent] = useState("");
  const [visibility, setVisibility] = useState<"public" | "keeper">("keeper");
  const [imageFile, setImageFile] = useState<File | null>(null);
  const [error, setError] = useState("");

  async function load() {
    const data = await api<{ items: CampaignBlock[] }>(
      `/campaigns/${campaignID}/blocks`,
    );
    setItems(data.items ?? []);
  }
  useEffect(() => {
    let active = true;
    api<{ items: CampaignBlock[] }>(`/campaigns/${campaignID}/blocks`)
      .then((data) => {
        if (active) setItems(data.items ?? []);
      })
      .catch(() => {
        if (active) setError("读取团本内容失败");
      });
    return () => {
      active = false;
    };
  }, [campaignID]);

  async function create(event: FormEvent) {
    event.preventDefault();
    try {
      let assetId: string | null = null;
      if (type === "image") {
        if (!imageFile) {
          setError("请选择图片");
          return;
        }
        assetId = (await uploadImage(campaignID, imageFile)).id;
      }
      await api(`/campaigns/${campaignID}/blocks`, {
        method: "POST",
        body: JSON.stringify({ type, title, content, visibility, assetId }),
      });
      setTitle("");
      setContent("");
      setImageFile(null);
      await load();
    } catch (reason) {
      setError(reason instanceof APIError ? reason.message : "新增内容失败");
    }
  }

  async function update(block: CampaignBlock) {
    try {
      await api(`/campaigns/${campaignID}/blocks/${block.id}`, {
        method: "PATCH",
        body: JSON.stringify(block),
      });
      await load();
    } catch (reason) {
      setError(reason instanceof APIError ? reason.message : "保存内容失败");
    }
  }

  async function move(blockID: string, direction: "up" | "down") {
    await api(`/campaigns/${campaignID}/blocks/${blockID}/move`, {
      method: "POST",
      body: JSON.stringify({ direction }),
    });
    await load();
  }

  async function remove(blockID: string) {
    if (!window.confirm("确定删除这个内容块吗？此操作目前不可恢复。")) return;
    await api(`/campaigns/${campaignID}/blocks/${blockID}`, {
      method: "DELETE",
    });
    await load();
  }

  function changeBlock(id: string, changes: Partial<CampaignBlock>) {
    setItems((current) =>
      current.map((item) => (item.id === id ? { ...item, ...changes } : item)),
    );
  }

  return (
    <section className="panel campaign-content">
      <div className="section-heading">
        <h2>团本内容</h2>
        {canManage && <small>Markdown 不允许原始 HTML</small>}
      </div>
      {error && <p className="error-banner">{error}</p>}
      <div className="campaign-blocks">
        {items
          .filter((block) => !playerPreview || block.visibility === "public")
          .map((block, index, visibleItems) =>
            canManage && !playerPreview ? (
              <article
                className={`campaign-block-editor visibility-${block.visibility}`}
                key={block.id}
              >
                <div className="block-toolbar">
                  <span>{blockTypeNames[block.type]}</span>
                  <select
                    value={block.visibility}
                    onChange={(event) =>
                      changeBlock(block.id, {
                        visibility: event.target
                          .value as CampaignBlock["visibility"],
                      })
                    }
                  >
                    <option value="public">公开</option>
                    <option value="keeper">仅 KP</option>
                  </select>
                  <button
                    type="button"
                    disabled={index === 0}
                    onClick={() => void move(block.id, "up")}
                  >
                    上移
                  </button>
                  <button
                    type="button"
                    disabled={index === visibleItems.length - 1}
                    onClick={() => void move(block.id, "down")}
                  >
                    下移
                  </button>
                  <button type="button" onClick={() => void remove(block.id)}>
                    删除
                  </button>
                </div>
                <input
                  value={block.title}
                  onChange={(event) =>
                    changeBlock(block.id, { title: event.target.value })
                  }
                  placeholder={
                    block.type === "heading" ? "标题（必填）" : "标题（可选）"
                  }
                />
                {block.type === "image" && block.assetId && (
                  <img
                    className="campaign-image"
                    src={assetURL(campaignID, block.assetId)}
                    alt={block.title || "团本图片"}
                  />
                )}
                {block.type !== "heading" && block.type !== "image" && (
                  <div className="markdown-editor">
                    <textarea
                      rows={7}
                      value={block.content}
                      onChange={(event) =>
                        changeBlock(block.id, { content: event.target.value })
                      }
                    />
                    <Markdown content={block.content} />
                  </div>
                )}
                <button
                  className="primary-button block-save"
                  type="button"
                  onClick={() => void update(block)}
                >
                  保存内容块
                </button>
              </article>
            ) : (
              <RenderedBlock block={block} key={block.id} />
            ),
          )}
        {items.length === 0 && <p className="muted">还没有团本内容。</p>}
      </div>
      {canManage && !playerPreview && (
        <form className="new-block" onSubmit={(event) => void create(event)}>
          <h3>新增内容块</h3>
          <div className="block-toolbar">
            <select
              value={type}
              onChange={(event) => setType(event.target.value as typeof type)}
            >
              <option value="heading">标题</option>
              <option value="text">文本</option>
              <option value="clue">线索</option>
              <option value="image">图片</option>
            </select>
            <select
              value={visibility}
              onChange={(event) =>
                setVisibility(event.target.value as typeof visibility)
              }
            >
              <option value="keeper">仅 KP</option>
              <option value="public">公开</option>
            </select>
          </div>
          <input
            value={title}
            onChange={(event) => setTitle(event.target.value)}
            placeholder={type === "heading" ? "标题（必填）" : "标题（可选）"}
            required={type === "heading"}
          />
          {type === "image" && (
            <label className="image-picker">
              选择图片（JPEG、PNG 或 WebP，最大 10 MB）
              <input
                type="file"
                accept="image/jpeg,image/png,image/webp"
                required
                onChange={(event) =>
                  setImageFile(event.target.files?.[0] ?? null)
                }
              />
            </label>
          )}
          {type !== "heading" && type !== "image" && (
            <div className="markdown-editor">
              <textarea
                rows={8}
                value={content}
                onChange={(event) => setContent(event.target.value)}
                placeholder="使用 Markdown 编写内容"
              />
              <Markdown content={content} />
            </div>
          )}
          <button className="primary-button" type="submit">
            添加到末尾
          </button>
        </form>
      )}
    </section>
  );
}

function RenderedBlock({ block }: { block: CampaignBlock }) {
  if (block.type === "heading")
    return <h2 className="content-heading">{block.title}</h2>;
  if (block.type === "image" && block.assetId)
    return (
      <figure className="rendered-image">
        <img
          className="campaign-image"
          src={assetURL(block.campaignId, block.assetId)}
          alt={block.title || "团本图片"}
        />
        {block.title && <figcaption>{block.title}</figcaption>}
      </figure>
    );
  return (
    <article className={`rendered-block rendered-${block.type}`}>
      {block.title && <h3>{block.title}</h3>}
      <Markdown content={block.content} />
    </article>
  );
}

async function uploadImage(
  campaignID: string,
  file: File,
): Promise<CampaignAsset> {
  const form = new FormData();
  form.append("file", file);
  const response = await fetch(`/api/v1/campaigns/${campaignID}/assets`, {
    method: "POST",
    credentials: "same-origin",
    body: form,
  });
  if (!response.ok) {
    const body = (await response.json().catch(() => null)) as {
      message?: string;
    } | null;
    throw new Error(body?.message ?? "上传图片失败");
  }
  return response.json() as Promise<CampaignAsset>;
}

function assetURL(campaignID: string, assetID: string) {
  return `/api/v1/campaigns/${campaignID}/assets/${assetID}`;
}

const blockTypeNames: Record<CampaignBlock["type"], string> = {
  heading: "标题",
  text: "文本",
  clue: "线索",
  image: "图片",
};

function Markdown({ content }: { content: string }) {
  return (
    <div className="markdown-preview">
      <ReactMarkdown remarkPlugins={[remarkGfm]}>
        {content || "*暂无内容*"}
      </ReactMarkdown>
    </div>
  );
}
